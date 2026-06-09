# sparsesvn 设计规格

- 日期：2026-06-09
- 状态：草案（待实现）
- 参考实现：`D:\download\checkout.rb`（Ruby 版本，2011 年由 Mark Seagrief 编写）

---

## 1. 项目定位与核心模型

**项目名**：`sparsesvn`（Go CLI）

**定位**：一个**水平触发、幂等**的 SVN 稀疏 checkout 管理工具。用户用一份 YAML 配置声明"我要这个仓库的哪些路径、各自什么深度"，工具负责让工作副本的实际稀疏状态**精确收敛**到配置描述的状态——无论起点是空目录、是已有的稀疏副本，还是配置发生了任意改动。

**核心心智模型**（类比 `terraform apply` / `kubectl apply`）：

```
desired state (sparsesvn.yaml) + current state (.svn/sparsesvn.state.yaml)
       v diff
   action plan (add / upgrade / downgrade / exclude)
       v execute
   svn checkout / update --set-depth ...
       v write
   new state file
```

**与原 Ruby 脚本 `checkout.rb` 的关键差异**：

- **去掉**：include 嵌套、从 svn 仓库读配置目录、平台过滤（linux/windows）、`@` / `*` 路径后缀魔法
- **新增**：状态对账机制——原脚本只升不降，新配置删除的路径会真正被 `exclude` 回收
- **新增**：`status` / `plan` / `validate` 辅助子命令、`--json` 输出、`--dry-run`、原子状态文件写入、半态失败恢复

---

## 2. YAML 配置 schema

**文件名约定**：默认 `sparsesvn.yaml`（位于 CLI 调用时的当前目录，或用 `-f` 指定）。

**Schema**：

```yaml
# sparsesvn.yaml
url: svn://server/repo/trunk    # 可选：SVN 仓库顶层 URL；CLI --url 覆盖此值
paths:
  - path: src/core              # 相对 url 的路径，必填
    depth: infinity             # empty | files | infinity，必填
  - path: src/utils
    depth: files
  - path: docs
    depth: empty
  - path: tests/unit
    depth: infinity
```

**字段规则**：

- `url`：可选。如果 YAML 没写，CLI 必须传 `--url`；两者都有时 CLI 覆盖。最终 url 写入状态文件。
- `paths[].path`：
  - 必填、非空字符串
  - 使用正斜杠 `/`（即使在 Windows 上）
  - 不允许前导 `/`、不允许 `..`、不允许尾部 `/`
  - 同一 `path` 不能重复出现（`validate` 时报错）
- `paths[].depth`：必填，枚举 `empty` / `files` / `infinity`
- **父目录自动占位**：用户列了 `src/core/utils`，CLI 自动确保 `src`、`src/core` 以 `empty`（或更深）拉取，无需手动列；但如果用户**显式**列了 `src` 且给了更深的 depth（如 `files`），以用户显式声明为准。

**校验时机**：`validate` 子命令、`apply` / `plan` / `status` 启动时都先校验，失败立即退出（退出码 2）。

**关于单文件抓取的说明**：SVN 的稀疏深度是**目录级**概念，没有"只取单个文件"的原生支持。要取单个文件，做法是把它的**父目录**设为 `files`（拉取父目录下所有直属文件）。CLI 不替用户做这层映射，配置就按 svn 原生语义来，避免歧义。

---

## 3. CLI 接口

### 全局标志（适用于所有子命令）

```
-f, --file PATH      配置文件路径（默认 ./sparsesvn.yaml）
-C, --workdir PATH   工作副本目录（默认当前目录）
-v, --verbose        增加日志详细度（可叠加：-v / -vv）
-q, --quiet          只输出错误
    --json           机器可读 JSON 输出（plan / status 支持）
    --no-color       禁用彩色输出
-h, --help / --version
```

### 子命令

**`sparsesvn apply [--url URL] [-r REV] [--dry-run]`**

让工作副本收敛到配置描述的状态。

- 工作副本不存在 -> 自动 `svn checkout --depth empty <url> <workdir>` 顶层后铺路径
- 工作副本存在 -> 读状态文件、diff、对账（add / upgrade / downgrade / exclude）
- `--url`：覆盖 YAML 中的 url；与状态文件中记录的 url 不一致时中止报错
- `-r, --revision REV`：传给 svn 的版本号（默认 `HEAD`），**不写入状态文件、不参与 hash 计算**
- `--dry-run`：等同于 `plan`
- 成功后写入新的状态文件

**`sparsesvn plan [--url URL] [-r REV]`**

打印将执行的动作清单，不调用任何 svn 命令。支持 `--json`。

**`sparsesvn status [--url URL]`**

显示"当前状态文件 vs 当前配置"的 diff，不连 svn、不执行任何修改。支持 `--json`。
退出码：`0` = 一致，`1` = 有差异，`2` = 错误（配置非法、url 不匹配等）。

**`sparsesvn validate`**

只校验 YAML 语法与字段合法性（schema、path 规则、depth 枚举、重复 path），不读状态文件、不连 svn。

### 退出码统一约定

```
0  成功 / 无差异
1  有差异（仅 status）/ 普通失败
2  用法错误、配置非法、url 不匹配
3  svn 命令执行失败
```

### 典型调用

```bash
# 首次拉取
sparsesvn apply --url svn://example.com/repo/trunk -C ./myrepo

# 改了 yaml 后
sparsesvn apply -C ./myrepo

# CI 中先看变化
sparsesvn plan -C ./myrepo --json | jq .

# 校验配置
sparsesvn validate -f ./sparsesvn.yaml
```

---
## 4. 幂等对账算法

### 输入

- `desired`：配置文件解析后的 `{path -> depth}` 映射
- `current`：状态文件解析后的 `{path -> depth}` 映射（状态文件不存在则视为空 map）

### 步骤

**1. 快速路径检查**

满足以下**全部**条件时打印"已是最新状态"并零 svn 调用退出：

- 状态文件存在
- `config_hash` 与当前配置内容 hash 一致
- url 未变
- 未指定 `-r` / `--revision`

**2. URL 一致性校验**

若状态文件存在且其 url 与最终 url 不同 -> 退出码 2 中止报错。

**3. 计算 desired 的完整路径集**

对 desired 中每条 `(path, depth)`，沿路径拆分父级，补齐父目录占位（默认 `empty`）。例如 `src/core/utils: infinity` -> 隐式加入 `src: empty`、`src/core: empty`（除非用户显式给了更深 depth，以用户为准）。结果记为 `expanded_desired`。

**4. 计算动作集**

遍历 `expanded_desired` 与 `current` 的并集：

| 条件 | 动作 |
|---|---|
| in desired, not in current | **ADD**（新增） |
| in both, depth 相同 | **NOOP**（跳过） |
| in both, desired > current（变深） | **UPGRADE** |
| in both, desired < current（变浅） | **DOWNGRADE** |
| in current, not in desired | **EXCLUDE** |

其中 depth 偏序：`empty < files < infinity`，`exclude` 视为"不存在"。

**5. 排序动作**

- ADD / UPGRADE：按路径深度**升序**（父先于子，svn 要求父目录已 checkout 才能操作子）
- DOWNGRADE / EXCLUDE：按路径深度**降序**（子先于父，避免 exclude 父目录时把子目录的状态搞乱）
- 同类动作内部按字典序，保证输出可预测

**6. 顶层 checkout 检查**

若工作副本根目录不是有效的 svn 工作副本（无 `.svn/`），先执行：

```
svn checkout --depth empty <url> <workdir>
```

然后在 workdir 中执行后续动作。

**7. 执行动作**

| 动作 | svn 命令 |
|---|---|
| ADD（depth = empty） | `svn update --set-depth empty --parents <path>`；若 svn 版本不支持 `--parents` 则 fallback 到沿途逐级 `--set-depth empty` |
| ADD（depth = files / infinity） | 父链占位由步骤 3 已展开为独立 ADD 动作，无需此处再补；本动作直接 `svn update --set-depth <depth> <path>`（前置父级 ADD 因步骤 5 升序排序已先执行） |
| UPGRADE / DOWNGRADE | `svn update --set-depth <new_depth> <path>` |
| EXCLUDE | `svn update --set-depth exclude <path>` |

若指定了 `-r REV`，所有 svn 命令均带 `-r REV`。

**8. 失败处理**

任一 svn 命令失败 -> 立即中止（不回滚已成功的动作）；**仍然写入"截至失败前已成功部分"的状态文件**，以便下次重跑能从实际状态继续对账。`config_hash` 字段在失败场景下写**空字符串**，避免下次被快速路径误判。退出码 3。

**9. 全部成功**

写入新状态文件（含新的 `config_hash`、`url`、`applied_at`、最终的 `expanded_desired`）。退出码 0。

### `-r REV` 的特殊处理

- revision 不写入状态文件、不参与 hash 计算
- 指定 `-r` 时**跳过步骤 1 的快速路径**
- 若 diff 算出的动作集为空但指定了 `-r`，仍会在工作副本根目录执行一次 `svn update -r REV`，确保 revision 对齐

---

## 5. 状态文件格式与读写

**位置**：`<workdir>/.svn/sparsesvn.state.yaml`

**格式**：

```yaml
# sparsesvn state file - DO NOT EDIT MANUALLY
version: 1
config_hash: "sha256:7f3a9c2e8b1d4f6a..."
url: "svn://server/repo/trunk"
applied_at: "2026-06-09T10:30:00Z"
paths:
  - { path: "src",            depth: empty }
  - { path: "src/core",       depth: infinity }
  - { path: "src/utils",      depth: files }
  - { path: "docs",           depth: empty }
  - { path: "tests",          depth: empty }
  - { path: "tests/unit",     depth: infinity }
```

**字段说明**：

- `version`：状态文件 schema 版本号。当前为 `1`，将来格式变动时用于读取时的兼容/迁移
- `config_hash`：上次成功 apply 时配置文件**内容**的 sha256（含 `sha256:` 前缀以便将来换算法）；失败半态写回时为**空字符串**
- `url`：上次 apply 时使用的最终 url
- `applied_at`：RFC3339 时间戳（UTC）
- `paths`：**展开后**的完整路径清单（含 CLI 自动补的父目录占位），按字典序排序，便于人工 diff

**读取规则**：

- 文件不存在 -> 视为"空状态"，继续 apply（按问题 7-II-1 决议）
- 文件存在但 YAML 解析失败 -> 退出码 2 中止，提示用户手动删除该文件以触发全量重建
- `version` 大于当前支持的最高版本 -> 退出码 2 中止，提示升级 CLI

**写入规则**：

- 写入采用 **临时文件 + 原子 rename**（`*.tmp` -> 目标名）避免写入过程中断导致状态文件损坏
- 仅在步骤 7 执行了至少一条 svn 命令后才写入（纯快速路径 no-op 不重写）
- 步骤 8 失败时也写入"截至失败前的实际状态"（仅包含已成功完成的动作所对应的最终状态），`config_hash` 字段在失败场景下写空字符串

---
## 6. Go 项目结构与模块边界

遵循"小而专、边界清"的原则，按职责拆分包，每个包都能独立理解和测试。

```
sparsesvn/
├── go.mod
├── go.sum
├── README.md
├── LICENSE
├── cmd/
│   └── sparsesvn/
│       └── main.go              # 入口，仅做：解析参数 -> 调用 cli 包
├── internal/
│   ├── cli/                     # 子命令注册与参数绑定（cobra）
│   │   ├── root.go              # 全局 flags、根命令
│   │   ├── apply.go
│   │   ├── plan.go
│   │   ├── status.go
│   │   └── validate.go
│   ├── config/                  # YAML 配置：解析 + 校验 + hash
│   │   ├── config.go            # Config / PathSpec 类型 + Load(path)
│   │   ├── validate.go          # 字段校验（path 规则、depth 枚举、重复）
│   │   └── hash.go              # 内容 sha256
│   ├── state/                   # 状态文件：读 / 写（原子 rename）/ 缺失处理
│   │   ├── state.go             # State 类型 + Load/Save
│   │   └── atomic.go            # 临时文件 + rename
│   ├── plan/                    # 对账算法：纯函数，不碰文件系统、不调 svn
│   │   ├── expand.go            # desired -> expanded（父级 empty 占位）
│   │   ├── diff.go              # (expanded, current) -> []Action
│   │   ├── sort.go              # Action 排序
│   │   └── action.go            # Action 类型（ADD/UPGRADE/DOWNGRADE/EXCLUDE）
│   ├── svn/                     # svn 命令封装：唯一与 svn 交互的层
│   │   ├── client.go            # Client 接口 + 实现（os/exec）
│   │   ├── commands.go          # Checkout/Update/Info 等方法
│   │   └── version.go           # 检测 svn 版本（用于 --parents fallback）
│   ├── executor/                # 串联 plan + svn + state，处理失败半态写回
│   │   └── executor.go
│   └── log/                     # 日志封装：verbose 级别、JSON 输出
│       └── log.go
└── test/
    ├── unit/                    # 各包单元测试（同包 *_test.go 优先）
    └── integration/             # 端到端测试，需本地 svnserve + 临时仓库
```

**关键边界**：

- **`plan` 包是纯函数**：输入 `Config + State`，输出 `[]Action`。不读文件、不调 svn。这让对账算法可以被密集单元测试覆盖，且 `plan` / `status` 子命令零副作用执行
- **`svn` 包是唯一调用 `svn` 二进制的地方**：定义 `Client` 接口，便于测试时注入 fake
- **`executor` 是唯一同时持有 `svn.Client` 和 `state.Writer` 的地方**：失败半态写回的策略集中在这里
- **`cli` 不知道 svn 细节**：只调 `executor.Apply(...)` / `plan.Compute(...)`

**依赖方向**（严格单向，避免循环）：

```
cli -> executor -> {plan, svn, state, log}
plan -> config (类型)
state -> config (类型)
所有包 -> log
```

**第三方依赖**（最小化）：

- `github.com/spf13/cobra` —— CLI 框架（成熟、广泛使用）
- `gopkg.in/yaml.v3` —— YAML 解析
- 标准库：`os/exec`、`crypto/sha256`、`encoding/json`、`log/slog`
- **不引入** 第三方 logger 库（用标准库 `log/slog`）、不引入断言库（标准库 `testing` 足够）

---

## 7. 测试策略

**单元测试**（快、无外部依赖）：

- **`config` 包**：YAML 解析正确性、各类非法 schema 报错（缺字段、非法 depth、重复 path、`..`/前导 `/`、空 paths 等）
- **`plan` 包**：对账算法的所有分支
  - 9 种 `(current, desired)` depth 组合的动作判定
  - 父级自动占位的展开逻辑（含"用户显式 depth 覆盖隐式 empty"的边界）
  - 排序：ADD/UPGRADE 升序、DOWNGRADE/EXCLUDE 降序
  - 空 current / 空 desired / 完全相同三种端况
- **`state` 包**：读写往返、文件不存在、YAML 损坏、version 过高、原子写入（写中途模拟崩溃后状态文件仍可读）
- **`svn` 包**：注入 fake 命令，验证生成的 svn 命令行参数与预期一致；version detection 的字符串解析

**集成测试**（慢、需要 svn 二进制）：

- 用 `svnadmin create` 在临时目录拉一个本地 file:// 仓库，预置若干目录/文件
- 跑端到端场景：
  1. 首次 apply（空 workdir -> 全新 checkout）
  2. 配置不变重跑（验证快速路径，零 svn 调用）
  3. 新增路径、升级 depth、降级 depth、排除路径
  4. URL 变更 -> 中止退出
  5. 状态文件丢失 -> 全量重建
  6. 模拟 svn 失败（路径不存在）-> 半态写回正确、退出码 3
  7. `-r REV` 指定旧版本 -> revision 对齐
  8. `plan` / `status` / `validate` 各子命令的输出与退出码
- 集成测试用 build tag `//go:build integration` 隔离，CI 中跑前先检查 `svn` 二进制可用，本地 `go test ./...` 默认跳过

**测试目标覆盖率**：`plan` / `config` / `state` 三个核心包 >= 85%；其他包 >= 60%。

**不写的测试**：CLI 参数解析（cobra 自己有测试）、log 输出格式（视觉细节，手工验证）。

---

## 8. 开发计划与里程碑

按依赖顺序分 4 个里程碑，每个里程碑结束都是可工作的状态：

**M1：核心算法（无副作用）**

- `config` 包（解析 + 校验 + hash）
- `state` 包（读写 + 原子写）
- `plan` 包（expand + diff + sort）
- 完整单元测试
- 此时还没有 CLI，但核心算法已可被测试验证

**M2：svn 集成与执行器**

- `svn` 包（命令封装 + version detection + fake 实现）
- `executor` 包（串联 + 半态写回）
- 用 fake svn client 的执行器单元测试

**M3：CLI 与日志**

- `cli` 包（4 个子命令 + 全局 flags）
- `log` 包（verbose 级别 + JSON 输出）
- 端到端可用：能从命令行跑 apply/plan/status/validate

**M4：集成测试与打磨**

- 基于本地 svnserve 的集成测试套件
- README（含 schema 文档、示例、与原 Ruby 脚本的迁移说明）
- 跨平台构建脚本（windows / linux / darwin × amd64 / arm64）
- 退出码、错误信息措辞统一审查

---

## 附录 A：决策记录

| 编号 | 决策 | 选择 |
|---|---|---|
| 1 | YAML schema | 全新设计，不兼容旧 Ruby 格式，无 include |
| 2 | 幂等权威源 | 状态文件法 |
| 3 | 路径收回操作 | `svn update --set-depth exclude` |
| 3b | svn 拒绝 exclude（本地修改）的处理 | 立即中止 |
| 4 | YAML schema 形态 | 扁平列表 + 显式 depth；无平台过滤；url 三者都支持 |
| 5 | CLI 主命令形态 | 单一 `apply`；辅助 `status` / `plan` / `validate` |
| 6 | 降级是否需要确认 | 不需要，直接执行 |
| 7a | 状态文件位置 | `<workdir>/.svn/sparsesvn.state.yaml` |
| 7b | URL 变化时行为 | 报错中止 |
| 7c | 状态文件缺失时行为 | 视为空状态全量对账 |
| 8a | svn 命令失败处理 | 立即中止，写入半态，退出码 3 |
| 8b | 日志详细度 | 默认中等（计划 + 命令摘要） |
| 8c | 输出格式 | 纯文本 + `--json` 选项 |
