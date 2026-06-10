# sparsesvn 安全上下文显示设计

## 问题陈述

当前 `sparsesvn` 的 `-C` 参数默认值为 `.`（当前目录），存在以下安全隐患：

1. 用户在错误目录运行命令时，程序不会提示
2. 如果当前目录是其他 SVN 仓库的工作副本，可能执行错误操作
3. CI/CD 脚本中，用户无法从命令行直观判断程序会在哪里执行

## 设计目标

1. **智能检测**：验证工作目录的有效性
2. **上下文显示**：始终显示执行环境信息
3. **安全第一**：防止误操作，不负责 svn switch
4. **CI/CD 友好**：无交互确认，输出清晰

## 设计方案

### 1. 智能检测逻辑

在 `PersistentPreRun` 中执行，所有命令继承：

```
输入: workdir（默认 "." 或用户指定的 -C）, configPath, urlOverride

Step 1: 验证 workdir
  - 检查 workdir 是否存在
    - 不存在 → 允许继续（可能是首次 checkout，会创建目录）
  - 检查 workdir/.svn 是否存在
    - 不存在：
      - 如果 -C 是默认值（"."）→ 报错
      - 如果 -C 是显式指定的 → 允许继续（首次 checkout）

Step 2: 加载配置获取 URL
  - 加载配置文件（configPath）
    - 失败 → 报错配置文件无效
  - 获取 finalURL = urlOverride || config.URL
    - 为空 → 报错 URL required

Step 3: URL 一致性检查（仅当 .svn 存在时）
  - 通过 svn info --show-item url 获取工作副本真实 URL（wcURL）
  - 比较 wcURL 和 finalURL
  - 不匹配 → 报错
  - 匹配 → 继续

Step 4: 存储信息到 GlobalFlags
  - Workdir（绝对路径）
  - FinalURL
  - ConfigPath
```

### 2. 默认值 vs 显式指定的区分

需要在 cobra 中标记 `-C` 是否为用户显式指定：

| 场景 | `.svn` 不存在时的行为 |
|------|----------------------|
| 未指定 `-C`（使用默认 `.`） | **报错**：要求用户显式指定 `-C` |
| 显式指定 `-C ./dir` | **允许**：视为首次 checkout |

实现方式：
- 使用 `cmd.Flags().Changed("workdir")` 检测用户是否显式设置了 `-C`

### 3. 信息显示格式

所有命令（apply、plan、status）执行时先输出上下文信息：

```
Working directory: D:\txcombo\project
Repository URL:    svn://svn.i.txcombo.com/c1/project/branches/c2trunk
Config file:       ./sparsesvn.yaml
```

然后接各自命令的原有输出。

### 4. 错误信息

| 场景 | 错误信息 |
|------|----------|
| 目录不存在 | `Error: working directory "xxx" does not exist` |
| 默认 -C 且无 .svn | `Error: current directory is not an SVN working copy. Use -C to specify working directory.` |
| 显式 -C 但 URL 不匹配 | `Error: URL mismatch. Working copy has "svn://repo-a", config specifies "svn://repo-b". Use "svn switch" to change the working copy URL, or update your config.` |

### 5. 静默模式

`-q` (quiet) 模式下：
- 仍然执行智能检测（安全检查不跳过）
- 不显示上下文信息
- 只显示命令核心输出或错误信息

## 实现方案

### 方案：CLI 层统一处理

在 `root.go` 的 `PersistentPreRun` 中添加检测逻辑，所有命令自动继承。

#### 修改文件

1. **internal/cli/root.go**
   - 添加 `WorkdirExplicit` 字段标记是否显式指定
   - 在 `PersistentPreRun` 中添加智能检测逻辑
   - 添加上下文信息显示函数

2. **internal/cli/apply.go**
   - 移除冗余的 URL 相关检查（已在 root 层处理）

3. **internal/cli/plan.go**
   - 同上

4. **internal/cli/status.go**
   - 同上

#### 新增函数

```go
// validateWorkdir 验证工作目录
func validateWorkdir(workdir string, isExplicit bool, configURL string) error {
    // 检查目录是否存在
    // 检查 .svn 是否存在
    // 如果 .svn 存在，检查 URL 一致性
}

// displayContext 显示上下文信息
func displayContext(out io.Writer, workdir, url, configFile string) {
    fmt.Fprintf(out, "Working directory: %s\n", workdir)
    fmt.Fprintf(out, "Repository URL:    %s\n", url)
    fmt.Fprintf(out, "Config file:       %s\n", configFile)
}
```

#### GlobalFlags 扩展

```go
type GlobalFlags struct {
    ConfigFile     string
    Workdir        string
    WorkdirExplicit bool   // 新增：标记是否显式指定
    Verbose        int
    Quiet          bool
    JSON           bool
    NoColor        bool
    ResolvedURL    string // 新增：解析后的 URL
}
```

## 测试场景

1. **正常场景**：在 SVN 工作副本目录运行
2. **首次 checkout**：显式指定 -C 到新目录
3. **错误目录**：在非工作副本目录运行（未指定 -C）
4. **URL 不匹配**：工作副本 URL 与配置不一致
5. **静默模式**：-q 参数下不显示上下文但执行检测
6. **JSON 输出**：--json 模式下的输出格式

## 不在范围内

- 不实现 `svn switch` 功能
- 不自动修复 URL 不匹配问题
- 不修改现有命令的核心逻辑
