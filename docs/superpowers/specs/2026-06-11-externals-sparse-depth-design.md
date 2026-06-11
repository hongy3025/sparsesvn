# sparsesvn externals 稀疏深度控制设计

- 日期：2026-06-11
- 状态：已实现
- 前置规格：`2026-06-09-sparsesvn-design.md`

---

## 1. 背景与目标

sparsesvn 目前完全不处理 SVN externals——执行 `svn checkout` / `svn update --set-depth` 时走 SVN 默认行为，自动拉取所有 `svn:externals` 引用的内容。这在稀疏 checkout 场景下有严重问题：

- `depth: infinity` 的路径若含 externals，会被隐式拉取，不受声明式管理控制
- externals 可能引入大目录或跨仓库依赖，违背稀疏 checkout 的初衷
- externals 不在状态文件中跟踪，幂等性可能被打破

**目标**：让 externals 支持与 paths 一致的声明式稀疏深度控制——在 YAML 配置中完全可配置，白名单模式，未声明的 externals 不拉取。

**范围约束**：
- 仅处理同仓库内 externals（用户确认这是主要场景）
- 不做 externals 的配置时自动发现校验（即不在 `validate` 子命令中扫描仓库校验"声明的 external 是否存在于 `svn:externals` 属性中"——这需要网络访问，属于 lint 功能，YAGNI）。运行时执行 external ADD 动作时会调用 `svn propget svn:externals` 获取 URL，这是必须的运行时操作，与配置校验不同
- 不改变现有 paths 的行为和语义

---

## 2. 方案选择

**选定方案：全局 `--ignore-externals` + 声明的 externals 逐个 checkout**

所有 `svn checkout` / `svn update` 命令加 `--ignore-externals` 阻止自动拉取。声明的 externals 在对账循环中作为独立动作，用 `svn checkout --depth <depth> <external_url> <local_path>` 单独拉取。

**否决方案**：
- 方案 B（`svn propget` 自动发现 + 选择性拉取）：引入"配置 vs 仓库实际"双重校验复杂度，YAGNI
- 方案 C（默认拉取 + 事后 exclude）：浪费带宽，违背声明式精神

---

## 3. YAML 配置 Schema 扩展

在 `paths` 条目中增加可选的 `externals` 子列表：

```yaml
url: svn://server/repo/trunk
paths:
  - path: src/core
    depth: infinity
    externals:                    # 可选，仅当该目录有 svn:externals 定义且需要拉取时声明
      - target: lib/utils        # external 在工作副本中的本地子目录名（相对 path）
        depth: files             # empty | files | infinity
      - target: lib/proto
        depth: infinity
  - path: docs
    depth: files                  # 无 externals 字段 = 不拉取该目录的任何 external
```

**字段规则**：

- `externals`：可选列表。省略 = 不拉取该 path 的任何 external
- `externals[].target`：必填，external 的本地挂载目录名，相对于父 `path`
  - 必须是单层目录名（不含 `/`、`..`、前导 `/`、尾部 `/`）
  - 同一 `path` 下 `target` 不能重复
- `externals[].depth`：必填，枚举 `empty` / `files` / `infinity`
- **约束**：父 `path` 的 `depth` 为 `empty` 时不允许声明 `externals`（校验时报错）

**设计决策**：

- `target` 是本地目录名而非 SVN URL。sparsesvn 在运行时通过 `svn propget svn:externals` 从工作副本获取 URL 映射，用户只需声明"哪个 external 子目录需要拉取、拉到什么深度"
- `target` 限制为单层目录名，因为 `svn:externals` 定义中每个条目就是一级子目录

---

## 4. 状态文件扩展

状态文件 `version` 从 `1` 升级到 `2`，`paths` 条目增加可选 `externals` 子列表：

```yaml
# sparsesvn state file - DO NOT EDIT MANUALLY
version: 2
config_hash: "sha256:7f3a9c2e..."
url: "svn://server/repo/trunk"
applied_at: "2026-06-11T10:30:00Z"
paths:
  - path: src/core
    depth: infinity
    externals:
      - target: lib/utils
        depth: files
      - target: lib/proto
        depth: infinity
  - path: docs
    depth: files
```

**兼容性**：
- 读取 version `1` 的状态文件时，`externals` 默认为空切片
- version `1` 升级后首次 apply 会因 config_hash 变化触发全量对账，自然为所有 paths 补充空的 externals 状态

**联动规则**：
- 父 path 被 EXCLUDE 时，其所有 externals 自动连带 EXCLUDE（无需单独动作）
- 父 path 从有 externals 变为无 externals 时，之前声明的 externals 全部 EXCLUDE

---

## 5. 对账算法扩展

### 5.1 Expand 阶段

`plan.Expand` 返回类型扩展：

```go
type ExpandResult struct {
    Paths     map[string]config.Depth
    Externals map[string][]ExternalSpec   // key = parent path
}

type ExternalSpec struct {
    Target string
    Depth  config.Depth
}
```

父目录自动占位逻辑不变。新增：若用户未为某 path 声明 externals，其 `Externals` 条目为空切片。

### 5.2 Diff 阶段

`plan.Diff` 在路径级 diff 基础上增加 external 级 diff。`Action` 类型扩展：

```go
type Action struct {
    Path      string           // 对普通 path 动作 = 路径; 对 external 动作 = ParentPath
    Kind      ActionKind       // ADD / UPGRADE / DOWNGRADE / EXCLUDE
    FromDepth config.Depth
    ToDepth   config.Depth
    External  *ExternalAction  // nil = 普通 path 动作; non-nil = external 动作
}

type ExternalAction struct {
    Target     string   // external 的本地目录名
    ParentPath string   // 所属的父 path（与 Action.Path 相同，冗余但便于独立访问）
}
```

**external 级 diff 规则**：

对每个 path，比较 `desired_externals` 与 `current_externals`：

| 条件 | 动作 |
|---|---|
| target in desired, not in current | **ADD** external |
| in both, depth 相同 | **NOOP** |
| in both, desired depth > current depth | **UPGRADE** external |
| in both, desired depth < current depth | **DOWNGRADE** external |
| target in current, not in desired | **EXCLUDE** external |

**联动规则**：
- 父 path 被升级/降级时，其 externals 不产生动作（external 深度与父 path 深度正交）
- 父 path 被 EXCLUDE 时，其所有 externals 自动生成 EXCLUDE 动作

### 5.3 Sort 阶段

排序规则不变，external 动作与普通 path 动作混合排序。external 动作的排序路径视为 `<parentPath>/<target>`，自然获得正确的深度排序：

- External ADD/UPGRADE：排在父 path ADD 之后、更深路径之前
- External EXCLUDE：排在子路径 EXCLUDE 之后、父 path EXCLUDE 之前

---

## 6. SVN 命令层与执行器变更

### 6.1 SVN 命令层

**所有现有命令加 `--ignore-externals`**：

| 函数 | 变更 |
|---|---|
| `Checkout()` | 加 `--ignore-externals` |
| `SetDepth()` | 加 `--ignore-externals` |
| `UpdateRoot()` | 加 `--ignore-externals` |
| `Exclude()` | 加 `--ignore-externals` |

**新增函数**：

```go
// ExternalDef 描述一个 svn:externals 条目
type ExternalDef struct {
    URL      string   // external 的源 URL
    Revision string   // 版本限定（空字符串表示无版本限定）
}

// GetExternals 读取工作副本中某目录的 svn:externals 属性
// 返回 {target: ExternalDef} 映射
func GetExternals(ctx context.Context, c Client, workdir, path string) (map[string]ExternalDef, error)

// CheckoutExternal 将 external 的源 URL checkout 到本地路径
func CheckoutExternal(ctx context.Context, c Client, workdir, parentPath, target, url string, depth config.Depth, extRevision, cliRevision string) error
```

`GetExternals` 执行：`svn propget svn:externals <path>`，解析输出为 target-URL 映射。

`svn:externals` 属性格式（每行）：
```
[-rN] URL target
target [-rN] URL
```
两种格式都需解析。`-rN` 为可选的版本限定。

**版本限定处理**：`svn:externals` 中的 `-rN` 是 externals 定义的一部分。`GetExternals` 解析时提取版本号，`CheckoutExternal` 执行时使用 `svn:externals` 中指定的版本（而非 CLI 的 `--revision` 参数）。这确保 externals 的版本与仓库定义一致。

`CheckoutExternal` 执行：`svn checkout --depth <depth> --ignore-externals [-r N] <url> <workdir>/<parentPath>/<target>`

### 6.2 执行器变更

`executor.Apply` 的核心变更：

1. **普通 path 动作和 external 动作混合排序后按序执行**
2. **External ADD 动作执行前**，调用 `svn.GetExternals()` 获取父目录的 externals 定义，解析出 target 对应的源 URL，再执行 `svn.CheckoutExternal()`
3. 若 target 在 `svn:externals` 定义中不存在，报错中止（退出码 3）
4. **External UPGRADE/DOWNGRADE**：`svn.SetDepth()`（external 的本地目录已是工作副本）
5. **External EXCLUDE**：`svn.Exclude()`

**性能考量**：`svn propget svn:externals` 是本地操作（读工作副本属性），不需要网络访问。

---

## 7. Go 类型与包结构变更

### 7.1 config 包

`config.go` 新增：

```go
type ExternalSpec struct {
    Target string
    Depth  Depth
}

// PathSpec 新增 Externals 字段
type PathSpec struct {
    Path      string
    Depth     Depth
    Externals []ExternalSpec
}
```

`rawPathSpec` / `rawExternalSpec` 对应新增。`Load()` 解析逻辑扩展。

`validate.go` 新增校验规则：
- `depth == empty` 时不允许 externals
- target 必须是单层目录名
- 同一 path 下 target 不重复

### 7.2 state 包

`state.go` 新增：

```go
type ExternalEntry struct {
    Target string
    Depth  config.Depth
}

// PathEntry 新增 Externals 字段
type PathEntry struct {
    Path      string
    Depth     config.Depth
    Externals []ExternalEntry
}
```

`StateVersion` 从 `1` 升到 `2`。读取 version `1` 时 `Externals` 默认空切片。

### 7.3 plan 包

`Action` 类型新增 `External *ExternalAction` 字段。`Expand` 返回 `ExpandResult`。`Diff` / `Sort` 支持 external 级动作。

### 7.4 svn 包

所有现有函数加 `--ignore-externals`。新增 `GetExternals()` 和 `CheckoutExternal()`。

### 7.5 executor 包

执行器处理 external 动作时调用 `GetExternals` 解析 URL，其余流程不变。

### 7.6 不新增包

所有变更在现有包内完成。

---

## 8. 测试策略补充

**单元测试新增**：

- **config 包**：externals 字段解析、`depth: empty` + externals 报错、target 格式校验、target 重复校验
- **plan 包**：external 级 diff（ADD/UPGRADE/DOWNGRADE/EXCLUDE）、父 path EXCLUDE 联动、external 动作排序
- **state 包**：version `1` 兼容读取、version `2` externals 读写往返

**集成测试新增**：

- 仓库中定义 `svn:externals` 属性
- 场景：声明 external 并 apply、未声明的 external 不拉取、external 深度升降级、external EXCLUDE、父 path EXCLUDE 联动

---

## 附录 A：决策记录

| 编号 | 决策 | 选择 |
|---|---|---|
| E1 | externals 管理模式 | 白名单 + 完全可配深度 |
| E2 | 未声明 externals 的行为 | 不拉取（全局 `--ignore-externals`） |
| E3 | 配置语法 | 嵌套式（paths 下嵌套 externals 列表） |
| E4 | external 源 URL 获取方式 | 运行时 `svn propget svn:externals` |
| E5 | target 字段语义 | 本地目录名（相对父 path），非 SVN URL |
| E6 | target 格式约束 | 单层目录名，不含 `/`、`..` |
| E7 | 状态文件版本升级 | `1` → `2`，向后兼容 |
| E8 | 父 path `depth: empty` 时的 externals | 校验时报错禁止 |
