# sparsesvn 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 实现 `sparsesvn` Go CLI，一个水平触发、幂等的 SVN 稀疏 checkout 管理工具，基于 YAML 配置让工作副本状态精确收敛到声明状态。

**架构：** 分层包结构（cli -> executor -> {plan, svn, state, config, log}）。`plan` 包是纯函数对账算法；`svn` 包是唯一与 svn 二进制交互的层；`executor` 串联三方处理失败半态写回。状态文件 `<workdir>/.svn/sparsesvn.state.yaml` 缓存上次成功的展开路径清单与配置 hash，支持快速路径检查。

**技术栈：** Go 1.22+、`github.com/spf13/cobra`（CLI 框架）、`gopkg.in/yaml.v3`（YAML 解析）、标准库 `log/slog` / `crypto/sha256` / `os/exec` / `encoding/json` / `testing`。

**规格引用：** `docs/superpowers/specs/2026-06-09-sparsesvn-design.md`

---

## 文件结构

| 文件 | 职责 |
|---|---|
| `go.mod`, `go.sum` | Go 模块定义与依赖锁 |
| `cmd/sparsesvn/main.go` | 入口；调用 `internal/cli` 根命令 |
| `internal/config/config.go` | `Config`、`PathSpec`、`Depth` 类型；`Load(path)` 函数 |
| `internal/config/validate.go` | `Validate(*Config)` 字段校验 |
| `internal/config/hash.go` | `HashFile(path)` 返回 `sha256:...` |
| `internal/state/state.go` | `State` 类型、`Load(workdir)`、`Save(workdir, *State)` |
| `internal/state/atomic.go` | `writeAtomic(path, data)` 临时文件 + rename |
| `internal/plan/action.go` | `Action` / `ActionKind` 类型 |
| `internal/plan/expand.go` | `Expand(*Config) map[string]Depth`，补齐父链 |
| `internal/plan/diff.go` | `Diff(expanded, current) []Action` |
| `internal/plan/sort.go` | `Sort([]Action)`：ADD/UPGRADE 升序、DOWNGRADE/EXCLUDE 降序 |
| `internal/svn/client.go` | `Client` 接口；`execClient` 实现 |
| `internal/svn/commands.go` | `Checkout`、`Update`、`IsWorkingCopy` 等方法 |
| `internal/svn/version.go` | `Version()` 检测 svn 版本（用于 `--parents` fallback） |
| `internal/svn/fake.go` | `FakeClient` 测试桩 |
| `internal/executor/executor.go` | `Apply(ctx, opts) Result`：串联 plan/svn/state，处理半态写回 |
| `internal/logx/logx.go` | 包装 `log/slog`，提供 verbose 级别 + 可选 JSON 输出（包名用 `logx` 避免与标准库冲突） |
| `internal/cli/root.go` | 根命令、全局 flags |
| `internal/cli/apply.go` | `apply` 子命令 |
| `internal/cli/plan.go` | `plan` 子命令 |
| `internal/cli/status.go` | `status` 子命令 |
| `internal/cli/validate.go` | `validate` 子命令 |
| `internal/cli/output.go` | plan/status 的文本与 JSON 输出格式化 |
| `README.md` | 用户文档：安装、schema、典型用法、与 Ruby 脚本的迁移说明 |
| `test/integration/integration_test.go` | 端到端集成测试，build tag `integration` |
| `.gitignore` | 忽略二进制、状态文件、IDE 文件 |
| `Makefile` | 构建、测试、跨平台打包目标 |

**注意命名调整**：包名用 `logx` 而非 `log`，避免与标准库 `log` 冲突。规格第 6 节里写的 `log/` 目录在实现时改为 `logx/`，对应包名 `logx`。

---

## 里程碑 M1：核心算法（无副作用）

实现 `config`、`state`、`plan` 三个纯逻辑包，完整单元测试。结束时尚无 CLI，但核心算法可通过 `go test ./internal/...` 验证。

### 任务 M1-1：初始化 Go 模块与构建工具

**文件：** `go.mod`、`.gitignore`、`Makefile`

- [ ] **步骤 1：初始化模块** `go mod init github.com/sparsesvn/sparsesvn`

- [ ] **步骤 2：写 `.gitignore`**

```
/sparsesvn
/sparsesvn.exe
/dist/
*.test
*.out
coverage.txt
.vscode/
.idea/
*.swp
```

- [ ] **步骤 3：写 `Makefile`**

```makefile
.PHONY: build test test-integration lint clean
build:
	go build -o sparsesvn ./cmd/sparsesvn
test:
	go test ./... -race -count=1
test-integration:
	go test -tags=integration ./test/integration/... -race -count=1 -v
lint:
	go vet ./...
clean:
	rm -f sparsesvn sparsesvn.exe coverage.txt
	rm -rf dist/
```

- [ ] **步骤 4：验证** `go build ./...` 预期无输出

- [ ] **步骤 5：Commit** `git add go.mod .gitignore Makefile && git commit -m "chore: init Go module and build tooling"`

---

### 任务 M1-2：config 包——Depth/PathSpec/Config 类型与 Load

**文件：** 创建 `internal/config/config.go`、`internal/config/config_test.go`

**类型签名：**
```go
type Depth int
const ( DepthEmpty Depth = iota; DepthFiles; DepthInfinity )
func (d Depth) String() string
func ParseDepth(s string) (Depth, error)

type PathSpec struct { Path string; Depth Depth }
type Config   struct { URL string; Paths []PathSpec }

func Load(path string) (*Config, error)
```

**实现要点：**
- 用 `gopkg.in/yaml.v3`；内部 `rawConfig` 用 `yaml` tag，再转换为强类型 `Config`
- `Load` 失败时错误带文件路径与原因（`fmt.Errorf` + `%w`）
- `ParseDepth` 仅接受 `empty`/`files`/`infinity`，其他报错

**测试用例：**
- `TestLoad_ValidMinimal`：写 yaml（url + 单条 path:src/depth:infinity），断言三字段
- `TestLoad_FileNotFound`：传不存在路径，err 非 nil
- `TestDepthString`：3 个枚举值的字符串

- [ ] **步骤 1：写测试**（以上 3 个用例）
- [ ] **步骤 2：运行** `go test ./internal/config/...` 预期 FAIL（编译错误）
- [ ] **步骤 3：实现 config.go**
- [ ] **步骤 4：拉依赖并测试** `go get gopkg.in/yaml.v3 && go mod tidy && go test ./internal/config/...` 预期 PASS
- [ ] **步骤 5：Commit** `git add go.mod go.sum internal/config/ && git commit -m "feat(config): add Config/PathSpec/Depth types and Load"`

---

### 任务 M1-3：config 包——Validate

**文件：** 创建 `internal/config/validate.go`、`internal/config/validate_test.go`；修改 `Load` 末尾调用 `Validate`

**接口签名：**
```go
func Validate(cfg *Config) error
```

**实现要点：**
- `cfg == nil` 或 `len(cfg.Paths) == 0` 报错
- 遍历 paths：调用内部 `validatePath`，并维护 `seen` map 检查重复
- `validatePath` 规则：非空 / 无反斜杠 / 无前导 `/` / 无尾部 `/` / 按 `/` 分段后无空段（拒绝 `//`）/ 无 `..` 段
- URL 字段不强制（可空，由 CLI 决定是否要求 `--url`）

**测试用例：**
- `TestValidate_OK`：合法配置
- `TestValidate_EmptyPaths`：paths 为空 -> err 含 "paths"
- `TestValidate_InvalidPath` (table-driven)：empty / leading-slash / trailing-slash / dot-dot / only-dot-dot / backslash / double-slash 每个都应报错
- `TestValidate_DuplicatePath`：两条相同 path -> err 含 "duplicate"
- `TestValidate_URLOptional`：URL 为空仍 OK

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 validate.go**
- [ ] **步骤 4：在 Load 末尾插入 `if err := Validate(cfg); err != nil { return nil, err }`**
- [ ] **步骤 5：运行** `go test ./internal/config/... -v` 预期全 PASS
- [ ] **步骤 6：Commit** `git add internal/config/ && git commit -m "feat(config): add Validate with path rules and duplicate detection"`

---

### 任务 M1-4：config 包——HashFile

**文件：** 创建 `internal/config/hash.go`、`internal/config/hash_test.go`

**接口签名：**
```go
func HashFile(path string) (string, error) // 返回 "sha256:<hex>"
```

**实现要点：**
- `crypto/sha256` + `io.Copy(h, file)` 流式
- 返回 `"sha256:" + hex.EncodeToString(sum)`

**测试用例：**
- `TestHashFile_Deterministic`：相同内容两次 hash 相同
- `TestHashFile_DifferentContent`：不同内容 hash 不同
- `TestHashFile_KnownVector`：空文件 hash == `"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"`
- `TestHashFile_NotFound`：err 非 nil

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 hash.go**
- [ ] **步骤 4：运行** `go test ./internal/config/...` 预期全 PASS
- [ ] **步骤 5：Commit** `git commit -am "feat(config): add HashFile (sha256)"`

---

### 任务 M1-5：state 包——类型与 atomic 写入原语

**文件：** 创建 `internal/state/state.go`（仅类型与常量）、`internal/state/atomic.go`、`internal/state/atomic_test.go`

**类型签名：**
```go
const StateVersion = 1
const StateFileRelPath = ".svn/sparsesvn.state.yaml"

type PathEntry struct {
    Path  string
    Depth config.Depth
}

type State struct {
    Version    int
    ConfigHash string    // 失败半态写回时为 ""
    URL        string
    AppliedAt  time.Time
    Paths      []PathEntry
}

func Path(workdir string) string // 返回 filepath.Join(workdir, StateFileRelPath)

// 内部原语
func writeAtomic(path string, data []byte, perm os.FileMode) error
```

**实现要点：**
- `writeAtomic`：在目标目录创建 `<base>.tmp.<pid>.<nano>`，写入，`f.Sync()`，`f.Close()`，`os.Rename` 到目标
- 失败时 `os.Remove` 清理临时文件（best-effort）
- 此任务**不实现** Load/Save，专心做底层

**测试用例：**
- `TestWriteAtomic_Success`：写入、读回、内容一致
- `TestWriteAtomic_Overwrites`：连写两次不同内容，第二次值生效
- `TestWriteAtomic_NoTempLeftOnSuccess`：成功后目录中不残留 `*.tmp.*`
- `TestWriteAtomic_DirNotExist`：父目录不存在时返回 error

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 state.go（类型 + Path 函数）和 atomic.go（writeAtomic）**
- [ ] **步骤 4：运行** `go test ./internal/state/...` 预期 PASS
- [ ] **步骤 5：Commit** `git add internal/state/ && git commit -m "feat(state): add State types and atomic write primitive"`

---

### 任务 M1-6：state 包——Load 与 Save

**文件：** 修改 `internal/state/state.go` 加入 Load/Save、新增 `internal/state/state_test.go`

**接口签名：**
```go
func Save(workdir string, s *State) error
func Load(workdir string) (*State, bool, error)
// 返回值：
//   (state, true, nil)  正常读到
//   (nil, false, nil)   文件不存在（视为空状态）
//   (nil, false, err)   解析失败或 version 过高
```

**实现要点：**
- yaml schema 严格匹配规格第 5 节字段名（`version` / `config_hash` / `url` / `applied_at` / `paths`）
- `paths` 每项 `{path, depth}`，depth 序列化为字符串（empty/files/infinity）
- `Load`：
  - `os.Stat` 不存在 -> 返回 `(nil, false, nil)`
  - 解析失败 -> 错误信息含 "consider deleting the state file to trigger full rebuild"
  - `version > StateVersion` -> 错误信息含 "please upgrade sparsesvn"
- `Save`：写入前在序列化结果前面拼上 `"# sparsesvn state file - DO NOT EDIT MANUALLY\n"` 注释行，调用 `writeAtomic`
- 写入前 `os.MkdirAll(filepath.Dir(path), 0755)` 确保 `.svn/` 已存在

**测试用例：**
- `TestSaveLoad_RoundTrip`：构造 State -> Save -> Load -> 字段全部一致（包括 AppliedAt 精度，注意时区，建议 Save 时统一转 UTC）
- `TestLoad_NotFound`：空 workdir，Load 返回 `(nil, false, nil)`
- `TestLoad_CorruptYAML`：写非法 yaml 到状态文件路径，Load err 非 nil 且含 "delete"
- `TestLoad_FutureVersion`：写 `version: 999` 的状态文件，Load err 含 "upgrade"
- `TestSave_PreservesOrder`：传入 paths 乱序，文件中按字典序写入
- `TestSave_CreatesStateDir`：workdir 下没有 `.svn/`，Save 仍成功（自动创建）

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 Load/Save**
- [ ] **步骤 4：运行** `go test ./internal/state/... -v` 预期全 PASS
- [ ] **步骤 5：Commit** `git commit -am "feat(state): add Load/Save with version check and corruption handling"`

---

### 任务 M1-7：plan 包——Action 类型

**文件：** 创建 `internal/plan/action.go`、`internal/plan/action_test.go`

**接口签名：**
```go
type ActionKind int
const (
    ActionAdd ActionKind = iota
    ActionUpgrade
    ActionDowngrade
    ActionExclude
)
func (k ActionKind) String() string // "add"/"upgrade"/"downgrade"/"exclude"

type Action struct {
    Kind      ActionKind
    Path      string
    FromDepth config.Depth // ActionAdd 时为 0 占位，无意义
    ToDepth   config.Depth // ActionExclude 时为 0 占位，无意义
}
```

**测试用例：**
- `TestActionKindString`：4 个枚举值字符串

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 action.go**
- [ ] **步骤 4：运行** `go test ./internal/plan/...` 预期 PASS
- [ ] **步骤 5：Commit** `git commit -am "feat(plan): add Action and ActionKind types"`

---

### 任务 M1-8：plan 包——Expand（展开父级 empty 占位）

**文件：** 创建 `internal/plan/expand.go`、`internal/plan/expand_test.go`

**接口签名：**
```go
// Expand 将 Config 展开为 {path -> depth} 完整映射，
// 自动为每条 path 的所有父级补 DepthEmpty 占位。
// 当用户显式列出某个父级且 depth 更深（>= empty）时，以用户显式声明为准。
func Expand(cfg *config.Config) map[string]config.Depth
```

**实现要点：**
- 先把用户显式声明的 `(path, depth)` 全部放入结果 map（同 path 不会重复，前置 Validate 保证）
- 再遍历每条 path，对其父链每一级，若结果 map 中**不存在**该父级，则加入 `(parent, DepthEmpty)`
- 父链拆分：`strings.Split(path, "/")`，从 `parts[0]` 累加到 `parts[len-2]`（不含自身）
- 若父级已显式存在（哪怕是 DepthEmpty），**不覆盖**用户显式声明

**测试用例：**
- `TestExpand_SinglePathInfinity`：`{src/core/utils: infinity}` -> `{src: empty, src/core: empty, src/core/utils: infinity}`
- `TestExpand_NoParents`：`{src: infinity}` -> `{src: infinity}`（无父级）
- `TestExpand_ExplicitParentDeeper`：`{src: files, src/core: infinity}` -> 保持 `src: files`（不被 empty 覆盖）
- `TestExpand_SiblingsShareParent`：`{src/a: files, src/b: infinity}` -> `{src: empty, src/a: files, src/b: infinity}`
- `TestExpand_EmptyConfig`：`cfg.Paths` 空 -> 返回空 map
- `TestExpand_ExplicitParentSameDepth`：`{src: empty, src/a: files}` -> `{src: empty, src/a: files}`（用户已声明 src）

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 expand.go**
- [ ] **步骤 4：运行** `go test ./internal/plan/... -v` 预期 PASS
- [ ] **步骤 5：Commit** `git commit -am "feat(plan): add Expand with parent placeholder auto-fill"`

---

### 任务 M1-9：plan 包——Diff（对账核心）

**文件：** 创建 `internal/plan/diff.go`、`internal/plan/diff_test.go`

**接口签名：**
```go
// Diff 比较 desired（expanded 后的）与 current（state.State.Paths 转 map 后）
// 返回未排序的动作集
func Diff(desired, current map[string]config.Depth) []Action
```

**实现要点：**（对照规格第 4 节步骤 4 决策表）
- 遍历 `desired`：
  - 若 `current` 无该 path -> 生成 `ActionAdd{Path, ToDepth: desired[p]}`
  - 若有 且 depth 相同 -> NOOP（不生成 Action）
  - 若有 且 desired > current -> `ActionUpgrade{Path, FromDepth, ToDepth}`
  - 若有 且 desired < current -> `ActionDowngrade{Path, FromDepth, ToDepth}`
- 遍历 `current`：
  - 若 `desired` 无该 path -> `ActionExclude{Path, FromDepth: current[p]}`
- depth 偏序由 `config.Depth` 的 int 值天然提供（empty=0 < files=1 < infinity=2）

**测试用例：**（覆盖 9 种 `(current, desired)` 组合 + 边界）
- `TestDiff_AllNew`：current 空 -> 所有 desired 变为 Add
- `TestDiff_AllRemoved`：desired 空 -> 所有 current 变为 Exclude
- `TestDiff_Identical`：两者相同 -> 返回空 slice
- `TestDiff_DepthChanges` (table-driven)：8 组 `(from, to)` 组合：
  - `empty -> files`：Upgrade
  - `empty -> infinity`：Upgrade
  - `files -> infinity`：Upgrade
  - `files -> empty`：Downgrade
  - `infinity -> empty`：Downgrade
  - `infinity -> files`：Downgrade
  - `empty -> empty` / `files -> files` / `infinity -> infinity`：NOOP（验证不出现在结果中）
- `TestDiff_MixedScenario`：综合场景含 add/upgrade/downgrade/exclude/noop 各一例

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 diff.go**
- [ ] **步骤 4：运行** `go test ./internal/plan/... -v` 预期 PASS
- [ ] **步骤 5：Commit** `git commit -am "feat(plan): add Diff (idempotent reconciliation)"`

---

### 任务 M1-10：plan 包——Sort（动作排序）

**文件：** 创建 `internal/plan/sort.go`、`internal/plan/sort_test.go`

**接口签名：**
```go
// Sort 按规格第 4 节步骤 5 规则就地排序：
//   ADD/UPGRADE 按路径深度（"/" 段数）升序，父先于子
//   DOWNGRADE/EXCLUDE 按路径深度降序，子先于父
//   同 Kind 同深度时按字典序
// 全局顺序：ADD/UPGRADE 先执行，DOWNGRADE/EXCLUDE 后执行
//   （这样不会出现"先 exclude 父再 add 子"的非法状态）
func Sort(actions []Action)
```

**实现要点：**
- 用 `sort.SliceStable`，比较函数：
  1. 按 Kind 分组：`{ADD, UPGRADE}` 为组 0，`{DOWNGRADE, EXCLUDE}` 为组 1，组 0 排前
  2. 组 0 内部：按 path 段数（`strings.Count(p, "/")`）升序，再按 path 字典序
  3. 组 1 内部：按 path 段数降序，再按 path 字典序
- 提供辅助 `func pathDepth(p string) int { return strings.Count(p, "/") }`

**测试用例：**
- `TestSort_AddsBeforeExcludes`：混入 add 和 exclude，验证 add 全部在前
- `TestSort_AddParentBeforeChild`：`[src/a/b, src/a, src]` 全是 Add -> 排序后 `[src, src/a, src/a/b]`
- `TestSort_ExcludeChildBeforeParent`：`[src, src/a, src/a/b]` 全是 Exclude -> 排序后 `[src/a/b, src/a, src]`
- `TestSort_LexicographicTieBreaker`：同深度同 Kind 多条按字典序
- `TestSort_Stable`：重复元素相对顺序保留
- `TestSort_EmptyAndSingle`：空 slice 和单元素不 panic

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 sort.go**
- [ ] **步骤 4：运行** `go test ./internal/plan/... -v` 预期 PASS
- [ ] **步骤 5：Commit** `git commit -am "feat(plan): add Sort with parent-first/child-first ordering"`

---

### M1 完工检查

- [ ] 运行：`go test ./internal/... -race -count=1 -v`
  - 预期：config、state、plan 三个包所有测试 PASS
- [ ] 运行：`go vet ./...` 预期无告警
- [ ] **审查检查点（子代理流程）**：M1 结束。检查覆盖率：`go test ./internal/... -coverprofile=coverage.txt && go tool cover -func=coverage.txt`，期望 plan / config / state 三包 >= 85%

---

## 里程碑 M2：svn 集成与执行器

实现 `svn` 包（封装 svn 命令）、`executor` 包（串联 plan + svn + state，处理半态写回）。结束时能在测试中用 fake svn client 走完整 apply 流程。

### 任务 M2-1：svn 包——Client 接口与版本检测

**文件：** 创建 `internal/svn/client.go`、`internal/svn/version.go`、`internal/svn/version_test.go`

**接口签名：**
```go
// Result 包含一次 svn 命令的执行结果
type Result struct {
    Args     []string
    Stdout   string
    Stderr   string
    ExitCode int
    Duration time.Duration
}

// Client 是与 svn 二进制交互的唯一接口
type Client interface {
    // Run 执行 svn <args...>，cwd 为工作目录
    Run(ctx context.Context, cwd string, args ...string) (*Result, error)
    // Version 返回 svn 版本号（major, minor, patch），用于能力检测
    Version(ctx context.Context) (major, minor, patch int, err error)
}

// NewExecClient 返回一个调用 PATH 中 `svn` 二进制的实现
func NewExecClient() Client
```

**实现要点：**
- `execClient` 用 `os/exec.CommandContext`
- 捕获 stdout/stderr 到 bytes.Buffer，转字符串
- 退出码：成功为 0；失败时取 `*exec.ExitError.ExitCode()`；进程未启动等返回 (nil, err)
- `Version()` 调用 `svn --version --quiet` 得到形如 `1.14.2\n` 的输出，正则 `^(\d+)\.(\d+)\.(\d+)` 解析

**测试用例：**
- `TestParseVersion_OK` (table-driven)：`"1.14.2\n"` -> `(1,14,2)`；`"1.10.0"` -> `(1,10,0)`；`"1.14.2 (r1899510)\n..."` -> `(1,14,2)`
- `TestParseVersion_Invalid`：`""`、`"not a version"` -> err 非 nil
- 不在此任务测试 `execClient.Run`（需要真实 svn，留给集成测试）；可单独测试 `parseVersion` 纯函数

**注意：** 把版本解析逻辑抽成纯函数 `parseVersion(s string) (int, int, int, error)`，便于单测；`Version()` 方法内部调 `Run` 后调 `parseVersion`。

- [ ] **步骤 1：写测试**（仅 parseVersion）
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 client.go（Client 接口 + execClient + NewExecClient）和 version.go（parseVersion + Version 方法）**
- [ ] **步骤 4：运行** `go test ./internal/svn/...` 预期 PASS
- [ ] **步骤 5：Commit** `git add internal/svn/ && git commit -m "feat(svn): add Client interface, exec impl, and version detection"`

---

### 任务 M2-2：svn 包——FakeClient（测试桩）

**文件：** 创建 `internal/svn/fake.go`、`internal/svn/fake_test.go`

**接口签名：**
```go
// FakeClient 是 Client 的内存实现，用于单元测试 executor 和 commands
type FakeClient struct {
    // Calls 按调用顺序记录所有 Run 调用
    Calls []FakeCall
    // VersionResponse：Version() 返回这个三元组
    VersionResponse struct { Major, Minor, Patch int; Err error }
    // FailOn：若 Args 匹配 FailOn 中的任一 pattern（按子串匹配），则返回 ExitCode=1 + Err
    FailOn []FakeFailRule
    // 默认 Run 返回 ExitCode=0, Stdout="", Err=nil
}

type FakeCall struct {
    Cwd  string
    Args []string
}

type FakeFailRule struct {
    ArgsContains []string // 所有元素都必须出现在 Args 中（顺序无关）
    Stderr       string
    ExitCode     int
}

func (f *FakeClient) Run(ctx context.Context, cwd string, args ...string) (*Result, error)
func (f *FakeClient) Version(ctx context.Context) (major, minor, patch int, err error)

// Reset 清空 Calls，便于测试间复用
func (f *FakeClient) Reset()
```

**测试用例：**
- `TestFakeClient_RecordsCalls`：连调三次 Run，Calls 长度 = 3 且参数一致
- `TestFakeClient_DefaultsToSuccess`：默认 Run 返回 ExitCode=0
- `TestFakeClient_FailOnMatches`：配置 FailOn 含 `["update", "src/nonexistent"]`，调用匹配时返回 ExitCode=1 + stderr
- `TestFakeClient_VersionResponse`：返回预设三元组

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 fake.go**
- [ ] **步骤 4：运行** `go test ./internal/svn/...` 预期 PASS
- [ ] **步骤 5：Commit** `git commit -am "feat(svn): add FakeClient for tests"`

---

### 任务 M2-3：svn 包——高层命令封装

**文件：** 创建 `internal/svn/commands.go`、`internal/svn/commands_test.go`

**接口签名：**
```go
// 顶层 checkout 空仓库
func Checkout(ctx context.Context, c Client, workdir, url string, revision string) error

// 对单条 path 设深度。revision 为 "" 时不传 -r
func SetDepth(ctx context.Context, c Client, workdir, path string, depth config.Depth, revision string) error

// 顶层 update -r REV，用于 -r 指定但动作集为空时的 revision 对齐
func UpdateRoot(ctx context.Context, c Client, workdir, revision string) error

// IsWorkingCopy 通过检查 workdir/.svn/ 目录是否存在判断
func IsWorkingCopy(workdir string) bool

// Exclude 是 SetDepth 的便捷形式，对应 svn 的 "exclude" 字符串（非 config.Depth 枚举值）
func Exclude(ctx context.Context, c Client, workdir, path string, revision string) error
```

**实现要点：**
- `Checkout`：调用 `c.Run(ctx, "", "checkout", "--depth", "empty", [-r REV,] url, workdir)`；注意 workdir 作为 svn 命令参数传入，所以 Run 的 cwd 可以是 "" 或临时目录
- `SetDepth`：根据 `depth.String()` 拼接 `[update, --set-depth, <depth>, [-r REV,] <path>]`；cwd = workdir
- `Exclude`：内部走 `c.Run(ctx, workdir, "update", "--set-depth", "exclude", [-r REV,] path)`
- 所有函数：调用后检查 `Result.ExitCode != 0` 返回带 stderr 的 error；err 用 `fmt.Errorf("svn %s failed: exit %d: %s", strings.Join(args," "), code, stderr)`
- `IsWorkingCopy`：`info, err := os.Stat(filepath.Join(workdir, ".svn")); return err == nil && info.IsDir()`

**测试用例：**（全部用 FakeClient 验证参数）
- `TestCheckout_BuildsArgs`：调用后 fake.Calls[0].Args == `[checkout, --depth, empty, svn://x, /tmp/w]`（HEAD 时不带 -r）
- `TestCheckout_WithRevision`：传 `revision="100"`，args 含 `-r 100`
- `TestSetDepth_Empty/Files/Infinity`：三个 depth 各一个 case
- `TestSetDepth_WithRevision`：args 含 `-r REV`
- `TestExclude_BuildsArgs`：args 末尾是 `--set-depth exclude <path>`
- `TestUpdateRoot_BuildsArgs`：args == `[update, -r, REV]`，cwd = workdir
- `TestIsWorkingCopy_True`：mkdir 临时 `.svn/` -> true
- `TestIsWorkingCopy_False`：空临时目录 -> false
- `TestSetDepth_FailingExitPropagatesError`：FakeClient 配置 FailOn 匹配，调用返回 err 含 "exit 1"

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 commands.go**
- [ ] **步骤 4：运行** `go test ./internal/svn/...` 预期 PASS
- [ ] **步骤 5：Commit** `git commit -am "feat(svn): add high-level Checkout/SetDepth/Exclude/UpdateRoot helpers"`

---

### 任务 M2-4：logx 包——日志封装

**文件：** 创建 `internal/logx/logx.go`、`internal/logx/logx_test.go`

**接口签名：**
```go
type Level int
const (
    LevelQuiet Level = iota // 只输出 error
    LevelNormal             // 默认：plan 摘要 + 命令简短结果
    LevelVerbose            // -v：每条 svn 命令
    LevelDebug              // -vv：每条 svn 命令的 stdout/stderr
)

type Logger struct { /* internal */ }

func New(out io.Writer, level Level, jsonMode bool) *Logger

func (l *Logger) Errorf(format string, args ...any)
func (l *Logger) Infof(format string, args ...any)    // LevelNormal+
func (l *Logger) Verbosef(format string, args ...any) // LevelVerbose+
func (l *Logger) Debugf(format string, args ...any)   // LevelDebug+
func (l *Logger) JSON(v any) error                    // 仅 jsonMode=true 时输出
```

**实现要点：**
- 内部用 `log/slog` 实现（或简单 fmt.Fprintf + level 比较）
- `jsonMode=true` 时：Errorf/Infof/Verbosef/Debugf 全部抑制；只有 `JSON` 方法生效（输出 `json.NewEncoder(out).Encode(v)`）
- `jsonMode=false` 时：`JSON` 方法等同 no-op
- 注意：Errorf 即使 LevelQuiet 也输出

**测试用例：**
- `TestLevels` (table-driven)：4 个 level × 4 个方法 = 16 case，验证输出/不输出
- `TestJSONMode_SuppressesText`：jsonMode=true 时 Infof 输出为空
- `TestJSONMode_EmitsJSON`：jsonMode=true 时 JSON({"k":"v"}) 输出 `{"k":"v"}\n`
- `TestNormalMode_NoJSON`：jsonMode=false 时 JSON 不输出任何东西

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 logx.go**
- [ ] **步骤 4：运行** `go test ./internal/logx/...` 预期 PASS
- [ ] **步骤 5：Commit** `git add internal/logx/ && git commit -m "feat(logx): add leveled logger with JSON mode"`

---

### 任务 M2-5：executor 包——主流程与 Result 类型

**文件：** 创建 `internal/executor/executor.go`、`internal/executor/executor_test.go`

**接口签名：**
```go
type Options struct {
    ConfigPath string             // YAML 配置路径，必填
    Workdir    string             // 工作副本目录，必填
    URLOverride string            // CLI --url；空字符串表示不覆盖
    Revision   string             // CLI -r REV；空字符串表示 HEAD（不传 -r 给 svn）
    DryRun     bool               // 若 true，只计算 plan 不执行 svn 命令
    Client     svn.Client         // 注入，便于测试
    Logger     *logx.Logger
}

type Result struct {
    Plan          []plan.Action     // 排序后的计划
    ExecutedCount int               // 实际执行的 svn 命令数（DryRun 时为 0）
    FastPath      bool              // 是否走快速路径退出
    StateAfter    *state.State      // 最终（或半态）状态
    FailedAction  *plan.Action      // 若失败，指向失败的 Action
    Err           error             // 业务错误（svn 失败、url 不匹配等）
}

// Apply 是核心入口：加载配置 -> 加载状态 -> 校验 url -> 快速路径 -> Expand -> Diff -> Sort
// -> 顶层 checkout（按需）-> 依次执行动作 -> 写状态
func Apply(ctx context.Context, opts Options) *Result

// Compute 只算计划不执行，用于 plan/status 子命令
func Compute(opts Options) (*Result, error)
```

**实现要点（按规格第 4 节算法分阶段）：**
1. 加载 config（`config.Load`），若失败返回 err（CLI 退出码 2）
2. 计算 finalURL：`opts.URLOverride > cfg.URL`；若两者都空 -> err "url required"
3. 加载 state（`state.Load`）
4. 若 state 存在 且 `state.URL != finalURL` -> 返回 err "url mismatch"（CLI 退出码 2）
5. 计算 configHash（`config.HashFile(opts.ConfigPath)`）
6. **快速路径**：state 存在 && `state.ConfigHash == configHash` && `opts.Revision == ""` -> 返回 `Result{FastPath: true, ExecutedCount: 0}`，不写状态
7. 调用 `plan.Expand` 得到 desired，构建 current map（from state.Paths 或空）
8. 调用 `plan.Diff` -> `plan.Sort` -> 得到 actions
9. **特殊情况**：actions 为空 && `opts.Revision != ""` -> 加一个特殊 action（在 UpdateRoot 阶段处理），或直接调用 `svn.UpdateRoot`；executedCount = 1，写状态
10. `opts.DryRun` -> 返回 Result 不执行
11. 若 `!svn.IsWorkingCopy(workdir)` -> 调 `svn.Checkout(...)` 顶层
12. **执行 actions**（按 Sort 顺序）：
    - ActionAdd / Upgrade / Downgrade -> `svn.SetDepth(..., action.ToDepth, ...)`
    - ActionExclude -> `svn.Exclude(...)`
    - 每条成功后 `executedCount++` 并把对应变化应用到内部 `current` map（add/upgrade/downgrade 设新 depth，exclude 删除）
    - 任一失败 -> 跳出循环，设 `FailedAction`、`Err`
13. 写状态：
    - 全部成功：`State{Version:1, ConfigHash:configHash, URL:finalURL, AppliedAt:now, Paths:fromCurrent}`
    - 失败半态：`State{Version:1, ConfigHash:"", URL:finalURL, AppliedAt:now, Paths:fromCurrent}`（注意 ConfigHash 空）
    - 写失败 -> 把 state.Save 的 err 包装进 Result.Err（但不覆盖更早的 FailedAction）

**注意：** Compute 等价于 Apply 的前 9 步（DryRun 模式），不写状态。

**测试用例（用 FakeClient）：**
- `TestApply_FreshCheckout`：空 workdir + 简单 config -> 验证 fake.Calls 包含顶层 checkout 和按顺序的 set-depth
- `TestApply_FastPath`：先 Apply 一次写入 state，再 Apply（config 未变）-> Result.FastPath=true, ExecutedCount=0, fake.Calls 没有新增
- `TestApply_URLMismatch`：state.URL 与 finalURL 不同 -> Result.Err 含 "url mismatch"，ExecutedCount=0
- `TestApply_URLRequired`：config 无 url，opts.URLOverride 也空 -> err
- `TestApply_AddNewPath`：state 中有 src（empty），config 新增 docs（empty）-> 只执行 1 条 svn 命令
- `TestApply_DowngradeAndExclude`：综合场景验证顺序（先 add/upgrade 后 downgrade/exclude）
- `TestApply_FailureWritesHalfState`：FakeClient 配置第 3 条失败 -> 前 2 条的状态写入 state 文件，ConfigHash="" 
- `TestApply_DryRun`：Result 含 Plan，但 fake.Calls 长度为 0，state 文件未写
- `TestApply_RevisionForcesUpdate`：state 完全匹配 + opts.Revision="100" -> 跳过快速路径，actions 为空时执行一次 UpdateRoot

- [ ] **步骤 1：写测试**（用 t.TempDir + FakeClient + 真实 state.Load/Save）
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 executor.go**
- [ ] **步骤 4：运行** `go test ./internal/executor/... -v` 预期全 PASS
- [ ] **步骤 5：Commit** `git add internal/executor/ && git commit -m "feat(executor): add Apply with idempotent reconciliation and half-state writeback"`

---

### M2 完工检查

- [ ] 运行：`go test ./internal/... -race -count=1`
  - 预期：所有包测试 PASS
- [ ] 运行：`go vet ./...` 无告警
- [ ] **审查检查点**：M2 结束。executor 已能用 fake client 完整跑通 apply 流程，下一里程碑只是接 CLI 入口。

---

## 里程碑 M3：CLI 与日志接入

实现 `cli` 包的根命令、4 个子命令、输出格式化。结束时能从命令行跑 `sparsesvn apply/plan/status/validate`。

### 任务 M3-1：cli 根命令与全局 flags

**文件：** 创建 `cmd/sparsesvn/main.go`、`internal/cli/root.go`

**接口签名：**
```go
// internal/cli/root.go
// Execute 是 main 调用的入口，返回 exit code
func Execute() int

// GlobalFlags 由根命令注册并被各子命令读取
type GlobalFlags struct {
    ConfigFile string // -f, --file，默认 "./sparsesvn.yaml"
    Workdir    string // -C, --workdir，默认 "."
    Verbose    int    // -v 计数（0/1/2）
    Quiet      bool   // -q
    JSON       bool   // --json
    NoColor    bool   // --no-color
}

// 根命令构造（cobra）
func newRootCmd() *cobra.Command
```

```go
// cmd/sparsesvn/main.go
package main

import (
    "os"
    "github.com/sparsesvn/sparsesvn/internal/cli"
)

func main() {
    os.Exit(cli.Execute())
}
```

**实现要点：**
- 用 `github.com/spf13/cobra`
- 根命令 `Use: "sparsesvn"`、`Short: "..."`、版本号由 `Version` 字段或 `--version` flag 提供（先硬编码 `"0.1.0-dev"`，后续改 ldflags 注入）
- `Execute()` 返回的 int：成功返回 0；cobra 错误返回 2；子命令通过 `cmd.RunE` 返回自定义 error 类型携带 exit code
- 定义私有 `type exitError struct { Code int; Err error }`，`Execute()` 检查 RunE 返回是否是 `*exitError`，取其 Code

**测试用例：**
- `TestExecute_NoArgs`：`os.Args = ["sparsesvn"]` -> 输出 help 并返回 0（cobra 默认行为）
- `TestExecute_UnknownCommand`：`["sparsesvn", "foobar"]` -> 返回 2
- `TestGlobalFlags_Defaults`：默认值正确
- `TestVerboseFlag_Counts`：`-v` -> Verbose=1，`-vv` -> Verbose=2

注意：cobra 测试需要捕获 stdout/stderr，参考 cobra 文档的 `cmd.SetOut/SetErr/SetArgs`。

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 cli/root.go 和 cmd/sparsesvn/main.go；`go get github.com/spf13/cobra && go mod tidy`**
- [ ] **步骤 4：运行** `go test ./internal/cli/...` 与 `make build` 预期 PASS 且生成 sparsesvn 二进制
- [ ] **步骤 5：手工验证** `./sparsesvn --help` 显示帮助，`./sparsesvn --version` 显示版本
- [ ] **步骤 6：Commit** `git add cmd/ internal/cli/ go.mod go.sum && git commit -m "feat(cli): add root command and global flags"`

---

### 任务 M3-2：cli validate 子命令

**文件：** 创建 `internal/cli/validate.go`、`internal/cli/validate_test.go`；在 root.go 注册

**接口签名：**
```go
func newValidateCmd(gf *GlobalFlags) *cobra.Command
```

**实现要点：**
- `RunE` 调用 `config.Load(gf.ConfigFile)`（Load 内部已含 Validate）
- 成功：打印 `"OK: <path> is valid"`；返回 nil（exit 0）
- 失败：返回 `&exitError{Code: 2, Err: err}`

**测试用例：**
- `TestValidate_OKConfig`：tmp dir 写合法 yaml -> stdout 含 "OK"，exit 0
- `TestValidate_InvalidYAML`：写非法 yaml -> exit 2，stderr 含错误
- `TestValidate_PathRuleViolation`：写含 `path: /src` 的 yaml -> exit 2

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 validate.go 并在 root.go 注册**
- [ ] **步骤 4：运行** `go test ./internal/cli/...` 预期 PASS
- [ ] **步骤 5：手工验证** `./sparsesvn validate -f /tmp/bad.yaml` 行为正确
- [ ] **步骤 6：Commit** `git commit -am "feat(cli): add validate subcommand"`

---

### 任务 M3-3：cli output 格式化（文本 + JSON）

**文件：** 创建 `internal/cli/output.go`、`internal/cli/output_test.go`

**接口签名：**
```go
func FormatPlan(actions []plan.Action) string

type PlanJSON struct {
    Url     string       `json:"url"`
    Actions []ActionJSON `json:"actions"`
    Summary SummaryJSON  `json:"summary"`
}

type ActionJSON struct {
    Kind      string `json:"kind"`
    Path      string `json:"path"`
    FromDepth string `json:"from_depth,omitempty"`
    ToDepth   string `json:"to_depth,omitempty"`
}

type SummaryJSON struct {
    Add       int `json:"add"`
    Upgrade   int `json:"upgrade"`
    Downgrade int `json:"downgrade"`
    Exclude   int `json:"exclude"`
    Total     int `json:"total"`
}

func BuildPlanJSON(url string, actions []plan.Action) PlanJSON
```

**实现要点：**
- 文本格式示例（用 `text/tabwriter` 对齐）：
  ```
  Plan: 5 actions (2 add, 1 upgrade, 1 downgrade, 1 exclude)

  + ADD       src                    -> empty
  + ADD       src/core               -> infinity
  ~ UPGRADE   docs        empty      -> files
  ~ DOWNGRADE old_module  infinity   -> empty
  - EXCLUDE   tmp         files
  ```
- JSON：`FromDepth` 在 Add 时省略；`ToDepth` 在 Exclude 时省略

**测试用例：**
- `TestFormatPlan_Empty`：空 actions -> "Plan: 0 actions (no changes)"
- `TestFormatPlan_AllKinds`：4 种 Kind 各 1 条，含正确标记符
- `TestBuildPlanJSON_Summary`：counts 正确
- `TestPlanJSON_Marshal`：json.Marshal 含预期字段（含 omitempty）

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 output.go**
- [ ] **步骤 4：运行** `go test ./internal/cli/...` 预期 PASS
- [ ] **步骤 5：Commit** `git commit -am "feat(cli): add plan text/JSON formatters"`

---

### 任务 M3-4：cli plan 子命令

**文件：** 创建 `internal/cli/plan.go`、`internal/cli/plan_test.go`；在 root.go 注册

**接口签名：** `func newPlanCmd(gf *GlobalFlags) *cobra.Command`

**子命令 flags：** `--url URL`、`-r, --revision REV`

**实现要点：**
- 构造 `executor.Options`，调 `executor.Compute(opts)`（注：Compute 是纯计算，不需要 client）
- 成功：根据 `gf.JSON` 输出文本或 JSON
- 失败：配置/url 错误 -> exit 2；其他 -> exit 1

**测试用例：**
- `TestPlan_TextOutput`：tmp dir 写 yaml，stdout 含 "Plan:"
- `TestPlan_JSONOutput`：加 `--json`，stdout 是合法 JSON
- `TestPlan_InvalidConfig`：exit 2

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 plan.go 并注册**
- [ ] **步骤 4：运行** `go test ./internal/cli/...` 预期 PASS
- [ ] **步骤 5：Commit** `git commit -am "feat(cli): add plan subcommand"`

---

### 任务 M3-5：cli status 子命令

**文件：** 创建 `internal/cli/status.go`、`internal/cli/status_test.go`；在 root.go 注册

**接口签名：** `func newStatusCmd(gf *GlobalFlags) *cobra.Command`

**实现要点：**
- 调 `executor.Compute(opts)` 得到 actions
  - `len(actions) == 0` -> "in sync"，exit 0
  - `len(actions) > 0` -> 同 plan 输出格式，exit 1
  - 错误 -> exit 2
- 支持 `--json`，多一字段 `"in_sync": bool`

**接口扩展：**
```go
type StatusJSON struct {
    PlanJSON
    InSync bool `json:"in_sync"`
}
```

**测试用例：**
- `TestStatus_InSync`：先 apply 后 status -> exit 0，含 "in sync"
- `TestStatus_HasDiff`：state 与 config 不同 -> exit 1
- `TestStatus_JSON_InSync`：JSON 含 `"in_sync": true`
- `TestStatus_URLMismatch`：state.URL ≠ config.URL -> exit 2

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 status.go 并注册**
- [ ] **步骤 4：运行** `go test ./internal/cli/...` 预期 PASS
- [ ] **步骤 5：Commit** `git commit -am "feat(cli): add status subcommand"`

---

### 任务 M3-6：cli apply 子命令

**文件：** 创建 `internal/cli/apply.go`、`internal/cli/apply_test.go`；在 root.go 注册

**接口签名：**
```go
func newApplyCmd(gf *GlobalFlags) *cobra.Command

// 测试友好的内部入口：接受 client 参数便于注入 FakeClient
func runApply(ctx context.Context, gf *GlobalFlags, applyFlags ApplyFlags, client svn.Client, out io.Writer, errOut io.Writer) int

type ApplyFlags struct {
    URL      string
    Revision string
    DryRun   bool
}
```

**实现要点：**
- cobra `RunE` 内调用 `runApply(ctx, gf, flags, svn.NewExecClient(), cmd.OutOrStdout(), cmd.ErrOrStderr())`，返回 int 包装为 `exitError`
- `runApply` 内：
  - 构造 `executor.Options{Client: client, Logger: logx.New(...)}`
  - `--dry-run` -> `opts.DryRun = true`
  - 调 `executor.Apply(ctx, opts)`
  - 处理结果：
    - `result.FastPath` -> "Already in sync"，return 0
    - `result.Err != nil`：
      - `result.FailedAction != nil` -> 打印失败 action + stderr，return 3
      - url 不匹配 / 配置错误 -> return 2
      - 其他 -> return 1
    - 成功 -> "Applied N actions in M.Ms"，return 0

**测试用例（用 FakeClient 注入）：**
- `TestApply_InvalidConfig`：exit 2
- `TestApply_URLRequired`：config 无 url 且无 --url -> exit 2
- `TestApply_DryRun_OutputsPlan`：dry-run 跑通 Compute 输出 plan，fake.Calls 长度为 0
- `TestApply_FreshCheckout_Success`：FakeClient 默认成功 -> exit 0，state 文件写入
- `TestApply_SvnFailure`：FakeClient FailOn 匹配 -> exit 3，半态写入

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 预期 FAIL
- [ ] **步骤 3：实现 apply.go 并注册**
- [ ] **步骤 4：运行** `go test ./internal/cli/...` 预期 PASS
- [ ] **步骤 5：手工验证** `./sparsesvn apply --help` 显示所有 flag
- [ ] **步骤 6：Commit** `git commit -am "feat(cli): add apply subcommand"`

---

### M3 完工检查

- [ ] 运行：`go test ./... -race -count=1` 预期全 PASS
- [ ] 运行：`make build` 生成二进制
- [ ] 手工冒烟：
  - `./sparsesvn --help` 列出 apply/plan/status/validate
  - 创建 `testdata/good.yaml`，`./sparsesvn validate -f testdata/good.yaml` exit 0
  - 创建 `testdata/bad.yaml`（含 `path: /src`），`./sparsesvn validate -f testdata/bad.yaml` exit 2
  - `./sparsesvn plan -f testdata/good.yaml -C /tmp/empty-dir` 输出 plan
- [ ] **审查检查点**：M3 结束，CLI 完整可用。

---

## 里程碑 M4：集成测试、文档与发布

实现基于本地 svnserve（或 file:// 协议）的端到端集成测试、README、跨平台构建。

### 任务 M4-1：集成测试基础设施

**文件：** 创建 `test/integration/helpers.go`、`test/integration/setup_test.go`（含 build tag `integration`）

**接口签名：**
```go
//go:build integration

// SvnRepo 表示一个本地测试用 svn 仓库
type SvnRepo struct {
    Path string // 仓库目录绝对路径
    URL  string // file:// 形式的 URL
}

// CreateTestRepo 在 t.TempDir 中创建一个 svn 仓库，预置如下结构：
//   /trunk/src/core/main.c
//   /trunk/src/core/util.c
//   /trunk/src/utils/helper.c
//   /trunk/docs/readme.md
//   /trunk/tests/unit/test_main.c
//   /trunk/tests/integration/test_api.c
// 通过 svnadmin create + svn import 实现
func CreateTestRepo(t *testing.T) *SvnRepo

// RequireSvnBinary 在所有集成测试开始前检查 svn 和 svnadmin 是否可用
func RequireSvnBinary(t *testing.T)

// RunCLI 构建并运行 sparsesvn 二进制，返回 stdout / stderr / exit code
func RunCLI(t *testing.T, args []string, workdir string) (stdout, stderr string, exitCode int)

// BuildBinary 用 go build 构建 sparsesvn 到 t.TempDir，返回二进制路径（缓存）
func BuildBinary(t *testing.T) string
```

**实现要点：**
- `CreateTestRepo`：
  1. `svnadmin create <tmpdir>/repo`
  2. 在 `<tmpdir>/import` 准备目录结构与文件（用 `os.MkdirAll` + `os.WriteFile`）
  3. `svn import <tmpdir>/import file://<tmpdir>/repo -m "init"`
  4. 返回 `SvnRepo{Path: <repo path>, URL: "file://" + abs path}`（Windows 上需要 `file:///C:/...` 三斜杠形式）
- `RequireSvnBinary`：`exec.LookPath("svn")` 和 `exec.LookPath("svnadmin")`，缺一就 `t.Skip("svn/svnadmin not found")`
- `BuildBinary`：用 `sync.Once` 缓存；调用 `go build -o <tmpdir>/sparsesvn ./cmd/sparsesvn`
- 文件第一行：`//go:build integration` + 空行 + `package integration`

**测试用例：**
- `TestCreateTestRepo_HasFiles`：创建仓库后用 `svn ls -R file://...` 验证 6 个文件存在
- `TestBuildBinary_Exists`：调用后二进制可执行
- `TestRunCLI_Version`：`RunCLI(["--version"])` exit=0 且 stdout 含版本号

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** `go test -tags=integration ./test/integration/...` 预期 FAIL
- [ ] **步骤 3：实现 helpers.go**
- [ ] **步骤 4：运行** 预期 PASS（前提：本机有 svn）
- [ ] **步骤 5：Commit** `git add test/integration/ && git commit -m "test(integration): add svn test repo helpers"`

---

### 任务 M4-2：集成测试——首次 apply 与快速路径

**文件：** 创建 `test/integration/apply_test.go`

**测试场景：**

```go
//go:build integration
package integration

func TestE2E_FreshApply(t *testing.T) {
    RequireSvnBinary(t)
    repo := CreateTestRepo(t)
    workdir := t.TempDir()
    
    yaml := fmt.Sprintf(`url: %s/trunk
paths:
  - path: src/core
    depth: infinity
  - path: docs
    depth: files
`, repo.URL)
    cfgPath := filepath.Join(workdir, "sparsesvn.yaml")
    os.WriteFile(cfgPath, []byte(yaml), 0644)
    
    stdout, stderr, code := RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
    if code != 0 {
        t.Fatalf("apply exit=%d stderr=%s", code, stderr)
    }
    
    // 验证：src/core/main.c 实际存在（infinity 拉下来了）
    assertFileExists(t, filepath.Join(workdir, "src/core/main.c"))
    // docs/readme.md 存在（files 拉了直属文件）
    assertFileExists(t, filepath.Join(workdir, "docs/readme.md"))
    // src/utils 不存在（未声明）
    assertFileNotExists(t, filepath.Join(workdir, "src/utils/helper.c"))
    // 状态文件存在
    assertFileExists(t, filepath.Join(workdir, ".svn/sparsesvn.state.yaml"))
}

func TestE2E_FastPath(t *testing.T) {
    // apply 两次，第二次输出含 "Already in sync" 且没有真正的 svn 调用
    // 用 stat() 检查状态文件 mtime 不变即可证明（或检查 stdout 含 "Already in sync"）
}
```

辅助函数：`assertFileExists`、`assertFileNotExists`。

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** `go test -tags=integration ./test/integration/... -run TestE2E_Fresh|TestE2E_FastPath -v`
- [ ] **步骤 3：必要时调试** `executor` / svn 命令行
- [ ] **步骤 4：通过后 Commit** `git commit -am "test(integration): e2e fresh apply and fast path"`

---

### 任务 M4-3：集成测试——增量场景（add / upgrade / downgrade / exclude）

**文件：** 修改 `test/integration/apply_test.go`

**测试场景：**

- `TestE2E_AddPath`：apply 后修改 yaml 加一条新 path，再 apply -> 新路径实际存在
- `TestE2E_UpgradeDepth`：先 `docs: empty`，apply 后改 `docs: files`，再 apply -> docs/readme.md 实际存在
- `TestE2E_DowngradeDepth`：先 `src/core: infinity`（main.c 存在），改 `src/core: empty`，再 apply -> main.c 被删除
- `TestE2E_ExcludePath`：先含 `tmp: infinity`（先用 svn import 准备 trunk/tmp/x.txt），改配置删除该条 -> tmp 目录被 exclude（不再存在）

每个测试独立创建 repo 和 workdir，避免相互污染。

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** 全部 PASS
- [ ] **步骤 3：Commit** `git commit -am "test(integration): e2e add/upgrade/downgrade/exclude scenarios"`

---

### 任务 M4-4：集成测试——错误场景

**文件：** 修改 `test/integration/apply_test.go`

**测试场景：**

- `TestE2E_URLMismatch`：先用 url-A apply，再改配置改用 url-B 跑 apply -> exit 2，stderr 含 "url mismatch"
- `TestE2E_StateMissing_FullRebuild`：apply 后删除 `.svn/sparsesvn.state.yaml`，再 apply -> 视为全空状态，重新执行所有 add（不会失败，svn 对"已是该 depth"是幂等的）
- `TestE2E_SvnFailure_HalfState`：配置含一条不存在路径（如 `path: nonexistent/dir`）+ 一条合法路径 -> apply exit 3，state 文件存在但 ConfigHash 为空，包含已成功的 path
- `TestE2E_RevisionAlignment`：apply 后用 `-r 1` 重跑 -> 即使动作集为空也执行一次 root update（用 `svn info` 验证 workdir revision == 1）

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** PASS
- [ ] **步骤 3：Commit** `git commit -am "test(integration): e2e error and edge-case scenarios"`

---

### 任务 M4-5：集成测试——plan / status / validate 输出

**文件：** 创建 `test/integration/cmd_test.go`

**测试场景：**

- `TestE2E_PlanText`：tmp workdir + yaml，跑 `plan`，stdout 含 "Plan: N actions" 且 N 等于预期
- `TestE2E_PlanJSON`：`plan --json`，stdout 解析为 PlanJSON 结构，actions 长度正确
- `TestE2E_StatusInSync`：apply 后 status exit 0，stdout 含 "in sync"
- `TestE2E_StatusHasDiff`：apply 后改 yaml，status exit 1
- `TestE2E_ValidateOK`：合法 yaml exit 0
- `TestE2E_ValidateBadPath`：含 `path: /abs` 的 yaml exit 2，stderr 含 "must not start with /"

- [ ] **步骤 1：写测试**
- [ ] **步骤 2：运行** PASS
- [ ] **步骤 3：Commit** `git commit -am "test(integration): e2e cmd output for plan/status/validate"`

---

### 任务 M4-6：README 与用户文档

**文件：** 创建 `README.md`

**内容大纲：**

1. **项目简介**：一段话定位 + 类比 terraform apply
2. **安装**：
   - `go install github.com/sparsesvn/sparsesvn/cmd/sparsesvn@latest`
   - 或从 releases 下载预编译二进制
   - 前置条件：`svn` 命令在 PATH 中（>= 1.6 支持 set-depth exclude）
3. **快速开始**：
   - 写一个 `sparsesvn.yaml` 示例
   - `sparsesvn apply --url svn://... -C ./myrepo`
4. **配置文件 schema**：完整字段说明，复制规格第 2 节
5. **子命令参考**：apply / plan / status / validate 各自的 flag、退出码、示例
6. **状态文件**：解释 `.svn/sparsesvn.state.yaml` 的作用、为何不应手动编辑、何时可安全删除（触发全量重建）
7. **典型工作流**：
   - 新克隆 -> `apply` 首次拉取
   - 改配置 -> `plan` 预览 -> `apply` 收敛
   - CI 流水线中用 `--json` 消费输出
8. **与原 Ruby `checkout.rb` 的迁移说明**：
   - 旧 schema vs 新 schema 对照
   - `@` / `*` 后缀 → 显式 `depth: files` / `depth: infinity`
   - `include` 嵌套 → 不再支持，请手工合并
   - 平台过滤 → 不再支持，可写两份 yaml 配合 CI 选择
9. **故障排查**：常见错误信息与解决办法
10. **License**：MIT 或 Apache-2.0（与原 Ruby 脚本 GPL-3 不同，因为是全新实现）

- [ ] **步骤 1：写 README.md**
- [ ] **步骤 2：人工审阅**（拼写、链接、代码块语法）
- [ ] **步骤 3：Commit** `git add README.md && git commit -m "docs: add README with schema, commands, and migration guide"`

---

### 任务 M4-7：跨平台构建脚本

**文件：** 修改 `Makefile`、创建 `scripts/release.sh`

**Makefile 新增 target：**

```makefile
.PHONY: dist
GOOS_LIST := linux darwin windows
GOARCH_LIST := amd64 arm64
VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS := -s -w -X main.version=$(VERSION)

dist: clean
	mkdir -p dist
	$(foreach os,$(GOOS_LIST),\
	  $(foreach arch,$(GOARCH_LIST),\
	    GOOS=$(os) GOARCH=$(arch) go build -ldflags "$(LDFLAGS)" \
	      -o dist/sparsesvn-$(os)-$(arch)$(if $(filter windows,$(os)),.exe) \
	      ./cmd/sparsesvn && \
	  )) \
	echo "Built: $$(ls dist/)"
```

**修改 `cmd/sparsesvn/main.go` 接受 version 注入：**
- 顶部加 `var version = "dev"`
- 传给 cli.Execute（修改 Execute 签名接受 version string）

**测试：**
- `make dist VERSION=0.1.0` -> dist/ 下有 6 个二进制
- `./dist/sparsesvn-linux-amd64 --version` 输出 `sparsesvn 0.1.0`（在 linux 上跑）

- [ ] **步骤 1：修改 Makefile 和 main.go**
- [ ] **步骤 2：本地跑** `make dist`，验证 6 个文件生成
- [ ] **步骤 3：在当前平台跑生成的二进制 `--version` 验证版本注入**
- [ ] **步骤 4：Commit** `git add Makefile cmd/ internal/cli/ && git commit -m "build: add cross-platform dist target with version injection"`

---

### 任务 M4-8：错误信息与退出码统一审查

**文件：** 审查 `internal/cli/*.go`、`internal/executor/executor.go`

**审查清单：**

- [ ] 所有 stderr 错误信息以小写动词开头（Go 惯例：`fmt.Errorf("read config: %w", err)`）
- [ ] 退出码按规格附录 A 决策表：0/1/2/3 用法一致
- [ ] 配置类错误（schema、url）-> 2
- [ ] svn 执行类错误 -> 3
- [ ] 其他业务错误 -> 1
- [ ] 用 `errors.Is` / 自定义 sentinel error 让 CLI 层准确分类
- [ ] error message 中文 vs 英文：统一英文（CLI 工具惯例；中文文档由 README 提供）

**新增：** 在 `internal/executor/errors.go` 定义 sentinel errors：

```go
package executor

import "errors"

var (
    ErrURLMismatch     = errors.New("url mismatch with state file")
    ErrURLRequired     = errors.New("url required: provide in config or --url flag")
    ErrConfigInvalid   = errors.New("config invalid")
    ErrSvnFailed       = errors.New("svn command failed")
    ErrStateCorrupt    = errors.New("state file corrupt; delete it to trigger full rebuild")
    ErrStateFutureVer  = errors.New("state file version newer than supported; please upgrade sparsesvn")
)
```

CLI 层用 `errors.Is(err, executor.ErrURLMismatch)` 判定退出码。

- [ ] **步骤 1：审查并修改各文件**
- [ ] **步骤 2：补充 sentinel errors 测试** 验证 `errors.Is` 链路通畅
- [ ] **步骤 3：运行** `go test ./... -race` 全 PASS
- [ ] **步骤 4：Commit** `git commit -am "refactor: unify error messages and exit codes via sentinel errors"`

---

### 任务 M4-9：CI 配置（可选）

**文件：** 创建 `.github/workflows/ci.yml`（若用 GitHub）；或 `.gitlab-ci.yml`（若用 GitLab）

**CI 步骤：**

```yaml
name: ci
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: sudo apt-get install -y subversion
      - run: go vet ./...
      - run: go test ./... -race -count=1 -coverprofile=coverage.txt
      - run: go test -tags=integration ./test/integration/... -race -v
      - uses: codecov/codecov-action@v4
        with: { file: coverage.txt }
  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: make dist VERSION=${{ github.ref_name }}
      - uses: actions/upload-artifact@v4
        with: { name: binaries, path: dist/ }
```

- [ ] **步骤 1：写 ci.yml**（根据用户实际平台选择 GitHub/GitLab/其他）
- [ ] **步骤 2：本地用 `act` 或推到远端验证一次**
- [ ] **步骤 3：Commit** `git add .github/ && git commit -m "ci: add GitHub Actions workflow for test and build"`

> 注：如果用户没有指定 CI 平台，此任务可标记为 optional 并跳过。

---

### M4 完工检查

- [ ] 运行：`go test ./... -race -count=1` 全 PASS
- [ ] 运行：`go test -tags=integration ./test/integration/... -v` 全 PASS（需本机有 svn）
- [ ] 运行：`make dist VERSION=0.1.0` 生成 6 个跨平台二进制
- [ ] 运行：`go test ./internal/... -coverprofile=coverage.txt && go tool cover -func=coverage.txt`
  - 期望：plan / config / state 三包 >= 85%；其他 >= 60%
- [ ] 手工冒烟：用本地 svnserve 真实仓库跑一遍完整流程（fresh apply -> 改 yaml -> apply -> status）
- [ ] **审查检查点**：M4 完工，项目可发布。

---

## 总览：任务依赖关系

```
M1-1 (Go module)
  ├── M1-2 (config types/Load)
  │     └── M1-3 (Validate) ──> M1-4 (Hash)
  ├── M1-5 (state types/atomic)
  │     └── M1-6 (Load/Save)
  └── M1-7 (Action) ──> M1-8 (Expand) ──> M1-9 (Diff) ──> M1-10 (Sort)

M2-1 (svn Client) ──> M2-2 (Fake) ──> M2-3 (commands)
M2-4 (logx)
[M1 + M2-3 + M2-4] ──> M2-5 (executor)

M3-1 (root) ──> M3-2 (validate) ─┐
              ──> M3-3 (output) ──> M3-4 (plan)
                                ──> M3-5 (status)
                                └─> M3-6 (apply)

M4-1 (integration helpers) ──> M4-2..M4-5 (e2e tests)
M4-6 (README) M4-7 (dist) M4-8 (errors) M4-9 (CI)
```

任务以串行思考写就，但 M2-1/M2-2/M2-3/M2-4 之间相对独立可少量并行；M3 的 4 个子命令也可独立开发。

---

## 自检

本计划文档作者自检：

- [x] 任务粒度合适：每个任务可在 1-3 小时内完成
- [x] 每个任务含明确的文件路径、接口签名、测试用例
- [x] 每个任务遵循 TDD：先写测试 -> 失败 -> 实现 -> 通过 -> commit
- [x] 每个任务有独立的 git commit 节点，回滚成本低
- [x] 每个里程碑结束有完工检查清单与审查检查点
- [x] 文件结构与规格文档一致（除 log → logx 的命名调整已说明）
- [x] 跨任务依赖关系已在末尾梳理
- [x] 覆盖了规格中的所有功能点（apply / plan / status / validate / dry-run / json / 状态文件半态 / url 校验）
- [x] 测试策略分层清晰：单元测试 (M1/M2/M3) + 集成测试 (M4)
- [x] 错误处理与退出码有专门审查任务 (M4-8)

---

## 风险与注意事项

### 风险 1：svn 命令行行为差异
- **缓解**：M2-1 的 Client 接口对 svn 命令行做抽象；集成测试用 file:// 协议避免网络依赖；README 注明最低 svn 版本要求 (1.6+)
- **遗留风险**：不同 svn 版本对 `--set-depth exclude` 行为可能微妙不同；建议在 CI 矩阵中测试 svn 1.10 / 1.14 两个版本

### 风险 2：Windows 路径处理
- **缓解**：全程用 `filepath` 而非 `path`；状态文件中存储的 path 始终用 `/` 分隔（POSIX 风格），写入 svn 时再用 `filepath.FromSlash` 转换
- **测试**：M1-9 的 diff 测试需包含跨平台路径用例

### 风险 3：原子写入在 Windows 上的可靠性
- **缓解**：M1-5 的 atomic write 用 `os.Rename`（Windows 上 `MoveFileEx` 是原子的）；写临时文件用 `os.CreateTemp` 在同一目录避免跨卷
- **测试**：M1-5 的测试在 Windows CI runner 上跑一遍

### 风险 4：集成测试对 svn 二进制的依赖
- **缓解**：`RequireSvnBinary` 检测缺失时 skip 而非 fail；CI 显式 `apt-get install subversion`
- **本地开发**：在 README 中说明如何在 Windows / macOS / Linux 安装 svn

### 风险 5：状态文件 schema 演进
- **缓解**：状态文件已有 `version: 1` 字段；M1-6 Load 时校验 version，未来 v2 时编写迁移逻辑
- **不兼容的 schema 变化**：通过 bump version 触发提示用户删除状态文件（视为全新仓库重建）

### 风险 6：用户配置中的相对路径与 workdir
- **缓解**：M1-3 Validate 校验 path 不以 `/` 开头、不含 `..`；M3-1 root 命令的 `-C / --chdir` 在执行任何子命令前 `os.Chdir`
- **测试**：M1-3 含恶意路径用例

---

## 实施建议

1. **严格按 M1 → M2 → M3 → M4 顺序推进**，不要跳跃
2. **每个任务的 TDD 流程不可省略**，先红再绿再 refactor
3. **每个 commit 前**：跑 `go test ./...`，跑 `go vet ./...`，跑 `gofmt -l .` 检查格式
4. **遇到 svn 行为不符合预期**：先写一个最小复现脚本，再回头改 Client 实现
5. **如果发现规格有遗漏**：先更新规格文档（specs/2026-06-09-sparsesvn-design.md），获得人工审批后再继续实现
6. **不要在实现期间扩大范围**：发现新需求记录到 `docs/superpowers/backlog.md`，留待下一轮设计

---

## 实施完成后的归档

- [ ] 将本计划文档移动到 `docs/superpowers/plans/archived/2026-06-09-sparsesvn-impl.md`
- [ ] 在 `docs/superpowers/specs/2026-06-09-sparsesvn-design.md` 头部加 "Status: Implemented" 标记
- [ ] 在 README.md 中加 "Status: v0.1.0 released"
- [ ] 用 `git tag v0.1.0` 打版本
- [ ] 如需发布到 GitHub Releases：`gh release create v0.1.0 dist/*`

---

**计划完成。审查通过后即可进入 M1-1 启动实现。**
