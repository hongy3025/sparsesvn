# Externals 稀疏深度控制 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 为 sparsesvn 添加 SVN externals 的声明式稀疏深度控制——白名单模式，未声明的 externals 不拉取，声明的 externals 支持 empty/files/infinity 三级深度。

**架构：** 所有 svn 命令加 `--ignore-externals` 阻止默认拉取。配置中声明的 externals 在对账循环中作为独立动作，通过 `svn propget svn:externals` 获取源 URL 后逐个 checkout。状态文件 version 升级到 2，向后兼容 version 1。

**技术栈：** Go 1.22+, spf13/cobra, gopkg.in/yaml.v3, os/exec

---

## 文件结构

| 操作 | 文件 | 职责 |
|------|------|------|
| 修改 | `internal/config/config.go` | 新增 `ExternalSpec` 类型，`PathSpec` 增加 `Externals` 字段，`Load()` 解析扩展 |
| 修改 | `internal/config/validate.go` | 新增 externals 校验规则（target 格式、重复、depth:empty 约束） |
| 修改 | `internal/state/state.go` | 新增 `ExternalEntry`，`PathEntry` 增加 `Externals`，version 升级到 2，读写逻辑扩展 |
| 修改 | `internal/plan/action.go` | `Action` 新增 `External` 字段，新增 `ExternalAction` 类型 |
| 修改 | `internal/plan/expand.go` | 返回类型改为 `ExpandResult`（含 Paths + Externals），保留 externals 信息 |
| 修改 | `internal/plan/diff.go` | 签名改为接受 `ExpandResult`，增加 external 级 diff 逻辑 |
| 修改 | `internal/plan/sort.go` | external 动作混合排序，排序路径用 `parentPath/target` |
| 修改 | `internal/svn/commands.go` | 所有命令加 `--ignore-externals`，新增 `GetExternals` 和 `CheckoutExternal` |
| 修改 | `internal/svn/fake.go` | 新增 `StdoutByArgs` 字段，支持按 args 匹配返回不同 Stdout（propget 场景需要） |
| 修改 | `internal/executor/executor.go` | 使用新的 `ExpandResult`，external 动作执行逻辑（propget + checkout），buildState 扩展 |
| 修改 | `internal/cli/output.go` | `FormatPlan` 和 `BuildPlanJSON` 支持 external 动作显示 |
| 新建 | `internal/config/config_test.go` | config 包单元测试 |
| 新建 | `internal/config/validate_test.go` | validate 包单元测试 |
| 新建 | `internal/state/state_test.go` | state 包单元测试 |
| 新建 | `internal/plan/expand_test.go` | expand 包单元测试 |
| 新建 | `internal/plan/diff_test.go` | diff 包单元测试 |
| 新建 | `internal/plan/sort_test.go` | sort 包单元测试 |
| 新建 | `internal/svn/externals_test.go` | externals 解析单元测试 |
| 新建 | `internal/executor/executor_test.go` | executor 包单元测试（含 external 动作） |

---

### 任务 1：config 包 — ExternalSpec 类型和 Load 解析

**文件：**
- 修改：`internal/config/config.go`
- 新建：`internal/config/config_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithExternals(t *testing.T) {
	yaml := `url: svn://server/repo/trunk
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
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sparsesvn.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(cfg.Paths))
	}
	p0 := cfg.Paths[0]
	if p0.Path != "src/core" {
		t.Errorf("path[0].Path = %q, want %q", p0.Path, "src/core")
	}
	if p0.Depth != DepthInfinity {
		t.Errorf("path[0].Depth = %v, want infinity", p0.Depth)
	}
	if len(p0.Externals) != 2 {
		t.Fatalf("path[0].Externals len = %d, want 2", len(p0.Externals))
	}
	if p0.Externals[0].Target != "lib/utils" {
		t.Errorf("externals[0].Target = %q, want %q", p0.Externals[0].Target, "lib/utils")
	}
	if p0.Externals[0].Depth != DepthFiles {
		t.Errorf("externals[0].Depth = %v, want files", p0.Externals[0].Depth)
	}
	if p0.Externals[1].Target != "lib/proto" {
		t.Errorf("externals[1].Target = %q", p0.Externals[1].Target)
	}
	if p0.Externals[1].Depth != DepthInfinity {
		t.Errorf("externals[1].Depth = %v, want infinity", p0.Externals[1].Depth)
	}
	p1 := cfg.Paths[1]
	if len(p1.Externals) != 0 {
		t.Errorf("path[1].Externals len = %d, want 0", len(p1.Externals))
	}
}

func TestLoadWithoutExternals(t *testing.T) {
	yaml := `url: svn://server/repo/trunk
paths:
  - path: src/core
    depth: infinity
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sparsesvn.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Paths[0].Externals) != 0 {
		t.Errorf("expected empty externals, got %d", len(cfg.Paths[0].Externals))
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/config/ -run TestLoadWith -v`
预期：编译失败（`ExternalSpec` 未定义）

- [ ] **步骤 3：编写最少实现代码**

在 `internal/config/config.go` 中：

1. 新增 `ExternalSpec` 类型（在 `PathSpec` 定义之前）：

```go
type ExternalSpec struct {
	Target string
	Depth  Depth
}
```

2. 修改 `PathSpec` 增加 `Externals` 字段：

```go
type PathSpec struct {
	Path      string
	Depth     Depth
	Externals []ExternalSpec
}
```

3. 新增 raw 类型：

```go
type rawExternalSpec struct {
	Target string `yaml:"target"`
	Depth  string `yaml:"depth"`
}
```

4. 修改 `rawPathSpec` 增加 `Externals` 字段：

```go
type rawPathSpec struct {
	Path      string           `yaml:"path"`
	Depth     string           `yaml:"depth"`
	Externals []rawExternalSpec `yaml:"externals"`
}
```

5. 修改 `Load()` 函数中 `cfg.Paths` 构建循环（约第 79-85 行），将：

```go
for i, rp := range raw.Paths {
    d, err := ParseDepth(rp.Depth)
    if err != nil {
        return nil, fmt.Errorf("load config %s: paths[%d]: %w", path, i, err)
    }
    cfg.Paths = append(cfg.Paths, PathSpec{Path: rp.Path, Depth: d})
}
```

改为：

```go
for i, rp := range raw.Paths {
    d, err := ParseDepth(rp.Depth)
    if err != nil {
        return nil, fmt.Errorf("load config %s: paths[%d]: %w", path, i, err)
    }
    ps := PathSpec{Path: rp.Path, Depth: d}
    for j, re := range rp.Externals {
        ed, err := ParseDepth(re.Depth)
        if err != nil {
            return nil, fmt.Errorf("load config %s: paths[%d].externals[%d]: %w", path, i, j, err)
        }
        ps.Externals = append(ps.Externals, ExternalSpec{Target: re.Target, Depth: ed})
    }
    cfg.Paths = append(cfg.Paths, ps)
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/config/ -run TestLoadWith -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add ExternalSpec type and parse externals from YAML"
```

---

### 任务 2：config 包 — externals 校验规则

**文件：**
- 修改：`internal/config/validate.go`
- 新建：`internal/config/validate_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// internal/config/validate_test.go
package config

import (
	"strings"
	"testing"
)

func TestValidateExternalsTargetEmpty(t *testing.T) {
	cfg := &Config{
		Paths: []PathSpec{
			{Path: "src", Depth: DepthInfinity, Externals: []ExternalSpec{
				{Target: "", Depth: DepthFiles},
			}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty target")
	}
	if !strings.Contains(err.Error(), "target must not be empty") {
		t.Errorf("error = %q, want mention of empty target", err)
	}
}

func TestValidateExternalsTargetHasSlash(t *testing.T) {
	cfg := &Config{
		Paths: []PathSpec{
			{Path: "src", Depth: DepthInfinity, Externals: []ExternalSpec{
				{Target: "sub/dir", Depth: DepthFiles},
			}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for target with slash")
	}
	if !strings.Contains(err.Error(), "must not contain '/'") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateExternalsTargetDotDot(t *testing.T) {
	cfg := &Config{
		Paths: []PathSpec{
			{Path: "src", Depth: DepthInfinity, Externals: []ExternalSpec{
				{Target: "..", Depth: DepthFiles},
			}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for target '..'")
	}
	if !strings.Contains(err.Error(), "must not be '..'") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateExternalsDuplicate(t *testing.T) {
	cfg := &Config{
		Paths: []PathSpec{
			{Path: "src", Depth: DepthInfinity, Externals: []ExternalSpec{
				{Target: "lib", Depth: DepthFiles},
				{Target: "lib", Depth: DepthInfinity},
			}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for duplicate target")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateExternalsWithEmptyDepth(t *testing.T) {
	cfg := &Config{
		Paths: []PathSpec{
			{Path: "src", Depth: DepthEmpty, Externals: []ExternalSpec{
				{Target: "lib", Depth: DepthFiles},
			}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for externals with depth:empty parent")
	}
	if !strings.Contains(err.Error(), "cannot declare externals") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateExternalsOK(t *testing.T) {
	cfg := &Config{
		Paths: []PathSpec{
			{Path: "src", Depth: DepthInfinity, Externals: []ExternalSpec{
				{Target: "lib", Depth: DepthFiles},
			}},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/config/ -run TestValidateExternal -v`
预期：FAIL（校验逻辑未实现）

- [ ] **步骤 3：编写最少实现代码**

在 `internal/config/validate.go` 的 `Validate` 函数中，在 `seen[p.Path] = i` 之后（第 24 行后）添加 externals 校验循环：

```go
		// Validate externals for this path
		if p.Depth == DepthEmpty && len(p.Externals) > 0 {
			return fmt.Errorf("paths[%d] %q: cannot declare externals when depth is empty", i, p.Path)
		}
		extSeen := make(map[string]int, len(p.Externals))
		for j, ext := range p.Externals {
			if ext.Target == "" {
				return fmt.Errorf("paths[%d] %q: externals[%d]: target must not be empty", i, p.Path, j)
			}
			if strings.Contains(ext.Target, "/") {
				return fmt.Errorf("paths[%d] %q: externals[%d]: target %q must not contain '/'", i, p.Path, j, ext.Target)
			}
			if ext.Target == ".." {
				return fmt.Errorf("paths[%d] %q: externals[%d]: target must not be '..'", i, p.Path, j)
			}
			if prev, ok := extSeen[ext.Target]; ok {
				return fmt.Errorf("paths[%d] %q: externals[%d]: target %q duplicate of externals[%d]", i, p.Path, j, ext.Target, prev)
			}
			extSeen[ext.Target] = j
		}
```

注意需要在文件头部确认 `"strings"` 已导入（当前已有）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/config/ -run TestValidateExternal -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/config/validate.go internal/config/validate_test.go
git commit -m "feat(config): add externals validation rules"
```

---

### 任务 3：state 包 — ExternalEntry 和 version 2

**文件：**
- 修改：`internal/state/state.go`
- 新建：`internal/state/state_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// internal/state/state_test.go
package state

import (
	"testing"
	"time"

	"github.com/hongy3025/sparsesvn/internal/config"
)

func TestSaveLoadWithExternals(t *testing.T) {
	dir := t.TempDir()
	s := &State{
		Version:    StateVersion,
		ConfigHash: "sha256:abc123",
		URL:        "svn://server/repo/trunk",
		AppliedAt:  time.Now().UTC().Truncate(time.Second),
		Paths: []PathEntry{
			{
				Path:  "src/core",
				Depth: config.DepthInfinity,
				Externals: []ExternalEntry{
					{Target: "lib/utils", Depth: config.DepthFiles},
					{Target: "lib/proto", Depth: config.DepthInfinity},
				},
			},
			{
				Path:      "docs",
				Depth:     config.DepthFiles,
				Externals: nil,
			},
		},
	}
	if err := Save(dir, s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, exists, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true")
	}
	if loaded.Version != StateVersion {
		t.Errorf("Version = %d, want %d", loaded.Version, StateVersion)
	}
	if len(loaded.Paths) != 2 {
		t.Fatalf("len(Paths) = %d, want 2", len(loaded.Paths))
	}
	p0 := loaded.Paths[0]
	if p0.Path != "src/core" {
		t.Errorf("Paths[0].Path = %q", p0.Path)
	}
	if len(p0.Externals) != 2 {
		t.Fatalf("Paths[0].Externals len = %d, want 2", len(p0.Externals))
	}
	if p0.Externals[0].Target != "lib/utils" {
		t.Errorf("Externals[0].Target = %q", p0.Externals[0].Target)
	}
	if p0.Externals[0].Depth != config.DepthFiles {
		t.Errorf("Externals[0].Depth = %v, want files", p0.Externals[0].Depth)
	}
	if p0.Externals[1].Target != "lib/proto" {
		t.Errorf("Externals[1].Target = %q", p0.Externals[1].Target)
	}
	if len(loaded.Paths[1].Externals) != 0 {
		t.Errorf("Paths[1].Externals len = %d, want 0", len(loaded.Paths[1].Externals))
	}
}

func TestLoadVersion1Compat(t *testing.T) {
	v1Yaml := "# sparsesvn state file - DO NOT EDIT MANUALLY\n" +
		"version: 1\n" +
		"config_hash: \"sha256:abc\"\n" +
		"url: \"svn://server/repo/trunk\"\n" +
		"applied_at: 2026-06-11T10:00:00Z\n" +
		"paths:\n" +
		"  - path: src\n" +
		"    depth: infinity\n" +
		"  - path: docs\n" +
		"    depth: files\n"
	dir := t.TempDir()
	statePath := Path(dir)
	os.MkdirAll(filepath.Dir(statePath), 0755)
	os.WriteFile(statePath, []byte(v1Yaml), 0644)

	loaded, exists, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true")
	}
	if loaded.Version != 1 {
		t.Errorf("Version = %d, want 1", loaded.Version)
	}
	for _, p := range loaded.Paths {
		if len(p.Externals) != 0 {
			t.Errorf("Path %q: expected empty externals, got %d", p.Path, len(p.Externals))
		}
	}
}
```

需要在 test 文件头部导入 `"os"` 和 `"path/filepath"`。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/state/ -run TestSaveLoadWith -v`
预期：编译失败（`ExternalEntry` 未定义，`PathEntry` 无 `Externals` 字段）

- [ ] **步骤 3：编写最少实现代码**

在 `internal/state/state.go` 中：

1. 新增 `ExternalEntry` 类型（在 `PathEntry` 之前）：

```go
type ExternalEntry struct {
	Target string
	Depth  config.Depth
}
```

2. 修改 `PathEntry` 增加 `Externals` 字段：

```go
type PathEntry struct {
	Path      string
	Depth     config.Depth
	Externals []ExternalEntry
}
```

3. 将 `StateVersion` 从 `1` 改为 `2`：

```go
const StateVersion = 2
```

4. 新增 raw 类型：

```go
type rawExternalEntry struct {
	Target string `yaml:"target"`
	Depth  string `yaml:"depth"`
}
```

5. 修改 `rawPathEntry` 增加 `Externals` 字段：

```go
type rawPathEntry struct {
	Path      string             `yaml:"path"`
	Depth     string             `yaml:"depth"`
	Externals []rawExternalEntry `yaml:"externals"`
}
```

6. 修改 `Save()` 函数中构建 `raw.Paths` 的逻辑（约第 68-70 行），将：

```go
for i, p := range sorted {
    raw.Paths[i] = rawPathEntry{Path: p.Path, Depth: p.Depth.String()}
}
```

改为：

```go
for i, p := range sorted {
    rpe := rawPathEntry{Path: p.Path, Depth: p.Depth.String()}
    for j, ext := range p.Externals {
        rpe.Externals = append(rpe.Externals, rawExternalEntry{Target: ext.Target, Depth: ext.Depth.String()})
    }
    raw.Paths[i] = rpe
}
```

7. 修改 `Load()` 函数中构建 `s.Paths` 的逻辑（约第 111-116 行），将：

```go
for i, rp := range raw.Paths {
    d, err := config.ParseDepth(rp.Depth)
    if err != nil {
        return nil, false, fmt.Errorf("parse state %s: paths[%d]: %w (consider deleting the state file to trigger full rebuild)", path, i, err)
    }
    s.Paths = append(s.Paths, PathEntry{Path: rp.Path, Depth: d})
}
```

改为：

```go
for i, rp := range raw.Paths {
    d, err := config.ParseDepth(rp.Depth)
    if err != nil {
        return nil, false, fmt.Errorf("parse state %s: paths[%d]: %w (consider deleting the state file to trigger full rebuild)", path, i, err)
    }
    pe := PathEntry{Path: rp.Path, Depth: d}
    for j, re := range rp.Externals {
        ed, err := config.ParseDepth(re.Depth)
        if err != nil {
            return nil, false, fmt.Errorf("parse state %s: paths[%d].externals[%d]: %w", path, i, j, err)
        }
        pe.Externals = append(pe.Externals, ExternalEntry{Target: re.Target, Depth: ed})
    }
    s.Paths = append(s.Paths, pe)
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/state/ -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/state/state.go internal/state/state_test.go
git commit -m "feat(state): add ExternalEntry, upgrade state version to 2"
```

---

### 任务 4：plan 包 — ExternalAction 和 ExpandResult

**文件：**
- 修改：`internal/plan/action.go`
- 修改：`internal/plan/expand.go`
- 新建：`internal/plan/expand_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// internal/plan/expand_test.go
package plan

import (
	"testing"

	"github.com/hongy3025/sparsesvn/internal/config"
)

func TestExpandWithExternals(t *testing.T) {
	cfg := &config.Config{
		URL: "svn://server/repo/trunk",
		Paths: []config.PathSpec{
			{
				Path:  "src/core",
				Depth: config.DepthInfinity,
				Externals: []config.ExternalSpec{
					{Target: "lib/utils", Depth: config.DepthFiles},
					{Target: "lib/proto", Depth: config.DepthInfinity},
				},
			},
			{Path: "docs", Depth: config.DepthFiles},
		},
	}
	result := Expand(cfg)

	// Check path expansion
	if result.Paths["src"] != config.DepthEmpty {
		t.Errorf("src depth = %v, want empty", result.Paths["src"])
	}
	if result.Paths["src/core"] != config.DepthInfinity {
		t.Errorf("src/core depth = %v, want infinity", result.Paths["src/core"])
	}
	if result.Paths["docs"] != config.DepthFiles {
		t.Errorf("docs depth = %v, want files", result.Paths["docs"])
	}

	// Check externals
	if len(result.Externals["src/core"]) != 2 {
		t.Fatalf("src/core externals len = %d, want 2", len(result.Externals["src/core"]))
	}
	if result.Externals["src/core"][0].Target != "lib/utils" {
		t.Errorf("externals[0].Target = %q", result.Externals["src/core"][0].Target)
	}
	if result.Externals["src/core"][0].Depth != config.DepthFiles {
		t.Errorf("externals[0].Depth = %v, want files", result.Externals["src/core"][0].Depth)
	}
	if len(result.Externals["docs"]) != 0 {
		t.Errorf("docs externals len = %d, want 0", len(result.Externals["docs"]))
	}
}

func TestExpandWithoutExternals(t *testing.T) {
	cfg := &config.Config{
		URL:   "svn://server/repo/trunk",
		Paths: []config.PathSpec{{Path: "src", Depth: config.DepthInfinity}},
	}
	result := Expand(cfg)
	if len(result.Externals["src"]) != 0 {
		t.Errorf("expected empty externals for src, got %d", len(result.Externals["src"]))
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/plan/ -run TestExpandWith -v`
预期：编译失败（`ExpandResult` 未定义，`Expand` 返回类型不匹配）

- [ ] **步骤 3：编写最少实现代码**

1. 在 `internal/plan/action.go` 中新增：

```go
type ExternalAction struct {
	Target     string
	ParentPath string
}
```

修改 `Action` 结构体，新增 `External` 字段：

```go
type Action struct {
	Kind      ActionKind
	Path      string
	FromDepth config.Depth
	ToDepth   config.Depth
	External  *ExternalAction
}
```

2. 在 `internal/plan/expand.go` 中：

新增类型和修改 `Expand` 返回值：

```go
type ExpandResult struct {
	Paths     map[string]config.Depth
	Externals map[string][]ExternalSpec
}

type ExternalSpec struct {
	Target string
	Depth  config.Depth
}

func Expand(cfg *config.Config) *ExpandResult {
	out := make(map[string]config.Depth, len(cfg.Paths)*2)
	extMap := make(map[string][]ExternalSpec, len(cfg.Paths))

	for _, p := range cfg.Paths {
		out[p.Path] = p.Depth
		var exts []ExternalSpec
		for _, e := range p.Externals {
			exts = append(exts, ExternalSpec{Target: e.Target, Depth: e.Depth})
		}
		if exts == nil {
			exts = []ExternalSpec{}
		}
		extMap[p.Path] = exts
	}
	for _, p := range cfg.Paths {
		parts := strings.Split(p.Path, "/")
		for i := 1; i < len(parts); i++ {
			parent := strings.Join(parts[:i], "/")
			if _, ok := out[parent]; !ok {
				out[parent] = config.DepthEmpty
			}
			if _, ok := extMap[parent]; !ok {
				extMap[parent] = []ExternalSpec{}
			}
		}
	}
	return &ExpandResult{Paths: out, Externals: extMap}
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/plan/ -run TestExpandWith -v`
预期：PASS

- [ ] **步骤 5：修复 executor.go 编译**

`executor.go` 第 86 行调用 `plan.Expand(cfg)` 返回类型变了，需要更新。将：

```go
desired := plan.Expand(cfg)
```

改为：

```go
expandResult := plan.Expand(cfg)
desired := expandResult.Paths
```

运行：`go build ./...`
预期：编译通过

- [ ] **步骤 6：Commit**

```bash
git add internal/plan/action.go internal/plan/expand.go internal/plan/expand_test.go internal/executor/executor.go
git commit -m "feat(plan): add ExternalAction, ExpandResult with externals support"
```

---

### 任务 5：plan 包 — external 级 Diff

**文件：**
- 修改：`internal/plan/diff.go`
- 新建：`internal/plan/diff_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// internal/plan/diff_test.go
package plan

import (
	"testing"

	"github.com/hongy3025/sparsesvn/internal/config"
)

func TestDiffExternalAdd(t *testing.T) {
	desired := map[string]config.Depth{"src": config.DepthInfinity}
	current := map[string]config.Depth{}
	desiredExt := map[string][]ExternalSpec{
		"src": {{Target: "lib", Depth: config.DepthFiles}},
	}
	currentExt := map[string][]ExternalSpec{
		"src": {},
	}
	actions := DiffWithExternals(desired, current, desiredExt, currentExt)
	found := false
	for _, a := range actions {
		if a.External != nil && a.External.Target == "lib" && a.Kind == ActionAdd {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ADD external action for lib, got %v", actions)
	}
}

func TestDiffExternalUpgrade(t *testing.T) {
	desired := map[string]config.Depth{"src": config.DepthInfinity}
	current := map[string]config.Depth{"src": config.DepthInfinity}
	desiredExt := map[string][]ExternalSpec{
		"src": {{Target: "lib", Depth: config.DepthInfinity}},
	}
	currentExt := map[string][]ExternalSpec{
		"src": {{Target: "lib", Depth: config.DepthFiles}},
	}
	actions := DiffWithExternals(desired, current, desiredExt, currentExt)
	found := false
	for _, a := range actions {
		if a.External != nil && a.External.Target == "lib" && a.Kind == ActionUpgrade {
			found = true
		}
	}
	if !found {
		t.Errorf("expected UPGRADE external action for lib, got %v", actions)
	}
}

func TestDiffExternalExclude(t *testing.T) {
	desired := map[string]config.Depth{"src": config.DepthInfinity}
	current := map[string]config.Depth{"src": config.DepthInfinity}
	desiredExt := map[string][]ExternalSpec{
		"src": {},
	}
	currentExt := map[string][]ExternalSpec{
		"src": {{Target: "lib", Depth: config.DepthFiles}},
	}
	actions := DiffWithExternals(desired, current, desiredExt, currentExt)
	found := false
	for _, a := range actions {
		if a.External != nil && a.External.Target == "lib" && a.Kind == ActionExclude {
			found = true
		}
	}
	if !found {
		t.Errorf("expected EXCLUDE external action for lib, got %v", actions)
	}
}

func TestDiffExternalParentExclude(t *testing.T) {
	// Parent path excluded -> all its externals auto-excluded
	desired := map[string]config.Depth{}
	current := map[string]config.Depth{"src": config.DepthInfinity}
	desiredExt := map[string][]ExternalSpec{}
	currentExt := map[string][]ExternalSpec{
		"src": {{Target: "lib", Depth: config.DepthFiles}},
	}
	actions := DiffWithExternals(desired, current, desiredExt, currentExt)
	found := false
	for _, a := range actions {
		if a.External != nil && a.External.Target == "lib" && a.Kind == ActionExclude {
			found = true
		}
	}
	if !found {
		t.Errorf("expected auto-EXCLUDE external action for lib when parent excluded, got %v", actions)
	}
}

func TestDiffExternalNoop(t *testing.T) {
	desired := map[string]config.Depth{"src": config.DepthInfinity}
	current := map[string]config.Depth{"src": config.DepthInfinity}
	desiredExt := map[string][]ExternalSpec{
		"src": {{Target: "lib", Depth: config.DepthFiles}},
	}
	currentExt := map[string][]ExternalSpec{
		"src": {{Target: "lib", Depth: config.DepthFiles}},
	}
	actions := DiffWithExternals(desired, current, desiredExt, currentExt)
	for _, a := range actions {
		if a.External != nil && a.External.Target == "lib" {
			t.Errorf("expected NOOP for lib, got %s", a.Kind)
		}
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/plan/ -run TestDiffExternal -v`
预期：编译失败（`DiffWithExternals` 未定义）

- [ ] **步骤 3：编写最少实现代码**

在 `internal/plan/diff.go` 中，保留原 `Diff` 函数（兼容），新增 `DiffWithExternals`：

```go
// DiffWithExternals computes path-level and external-level diff.
// desiredExt and currentExt map parent path -> external specs.
func DiffWithExternals(
	desired, current map[string]config.Depth,
	desiredExt, currentExt map[string][]ExternalSpec,
) []Action {
	actions := Diff(desired, current)

	// For each path present in both desired and current, diff externals
	for path, desiredExts := range desiredExt {
		if _, inDesired := desired[path]; !inDesired {
			continue // parent excluded, handled below
		}
		currentExts := currentExt[path]
		actions = append(actions, diffExternals(path, desiredExts, currentExts)...)
	}

	// Parent path excluded -> auto-exclude all its externals
	for path, currentExts := range currentExt {
		if _, inDesired := desired[path]; !inDesired {
			for _, ext := range currentExts {
				actions = append(actions, Action{
					Kind:      ActionExclude,
					Path:      path,
					FromDepth: ext.Depth,
					External:  &ExternalAction{Target: ext.Target, ParentPath: path},
				})
			}
		}
	}

	return actions
}

// diffExternals computes external-level diff for a single parent path.
func diffExternals(parentPath string, desired, current []ExternalSpec) []Action {
	dMap := make(map[string]config.Depth, len(desired))
	for _, e := range desired {
		dMap[e.Target] = e.Depth
	}
	cMap := make(map[string]config.Depth, len(current))
	for _, e := range current {
		cMap[e.Target] = e.Depth
	}

	var actions []Action
	for target, dDepth := range dMap {
		cDepth, ok := cMap[target]
		if !ok {
			actions = append(actions, Action{
				Kind:     ActionAdd,
				Path:     parentPath,
				ToDepth:  dDepth,
				External: &ExternalAction{Target: target, ParentPath: parentPath},
			})
			continue
		}
		if dDepth == cDepth {
			continue
		}
		if dDepth > cDepth {
			actions = append(actions, Action{
				Kind:      ActionUpgrade,
				Path:      parentPath,
				FromDepth: cDepth,
				ToDepth:   dDepth,
				External:  &ExternalAction{Target: target, ParentPath: parentPath},
			})
		} else {
			actions = append(actions, Action{
				Kind:      ActionDowngrade,
				Path:      parentPath,
				FromDepth: cDepth,
				ToDepth:   dDepth,
				External:  &ExternalAction{Target: target, ParentPath: parentPath},
			})
		}
	}
	for target, cDepth := range cMap {
		if _, ok := dMap[target]; !ok {
			actions = append(actions, Action{
				Kind:      ActionExclude,
				Path:      parentPath,
				FromDepth: cDepth,
				External:  &ExternalAction{Target: target, ParentPath: parentPath},
			})
		}
	}
	return actions
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/plan/ -run TestDiffExternal -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/plan/diff.go internal/plan/diff_test.go
git commit -m "feat(plan): add DiffWithExternals for external-level diff"
```

---

### 任务 6：plan 包 — external 动作排序

**文件：**
- 修改：`internal/plan/sort.go`
- 新建：`internal/plan/sort_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// internal/plan/sort_test.go
package plan

import (
	"testing"

	"github.com/hongy3025/sparsesvn/internal/config"
)

func TestSortWithExternals(t *testing.T) {
	actions := []Action{
		{Kind: ActionAdd, Path: "src/core", ToDepth: config.DepthInfinity},
		{Kind: ActionAdd, Path: "src/core", ToDepth: config.DepthFiles, External: &ExternalAction{Target: "lib", ParentPath: "src/core"}},
		{Kind: ActionAdd, Path: "src", ToDepth: config.DepthEmpty},
	}
	Sort(actions)
	// src (depth 0) should come before src/core (depth 1)
	// src/core (depth 1) should come before src/core/lib (effective depth 2)
	if actions[0].Path != "src" {
		t.Errorf("actions[0].Path = %q, want src", actions[0].Path)
	}
	if actions[1].Path != "src/core" || actions[1].External != nil {
		t.Errorf("actions[1] = %v, want src/core path action", actions[1])
	}
	if actions[2].External == nil || actions[2].External.Target != "lib" {
		t.Errorf("actions[2] = %v, want src/core/lib external action", actions[2])
	}
}

func TestSortExcludeWithExternals(t *testing.T) {
	actions := []Action{
		{Kind: ActionExclude, Path: "src", FromDepth: config.DepthInfinity},
		{Kind: ActionExclude, Path: "src/core", FromDepth: config.DepthFiles, External: &ExternalAction{Target: "lib", ParentPath: "src/core"}},
		{Kind: ActionExclude, Path: "src/core", FromDepth: config.DepthInfinity},
	}
	Sort(actions)
	// Deeper first: src/core/lib external (depth 2), then src/core (depth 1), then src (depth 0)
	if actions[0].External == nil || actions[0].External.Target != "lib" {
		t.Errorf("actions[0] = %v, want external exclude first", actions[0])
	}
	if actions[1].Path != "src/core" || actions[1].External != nil {
		t.Errorf("actions[1] = %v, want src/core path exclude", actions[1])
	}
	if actions[2].Path != "src" {
		t.Errorf("actions[2].Path = %q, want src", actions[2].Path)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/plan/ -run TestSortWith -v`
预期：FAIL（external 动作排序路径未使用 `parentPath/target`）

- [ ] **步骤 3：编写最少实现代码**

修改 `internal/plan/sort.go` 中的 `Sort` 函数，在 `pathDepth` 和路径比较中使用 external 动作的有效路径：

将 `Sort` �体替换为：

```go
func Sort(actions []Action) {
	sort.SliceStable(actions, func(i, j int) bool {
		a, b := actions[i], actions[j]
		aGroup := groupOf(a.Kind)
		bGroup := groupOf(b.Kind)
		if aGroup != bGroup {
			return aGroup < bGroup
		}
		aSortPath := sortPath(a)
		bSortPath := sortPath(b)
		aDepth := pathDepth(aSortPath)
		bDepth := pathDepth(bSortPath)
		if aGroup == 0 {
			if aDepth != bDepth {
				return aDepth < bDepth
			}
		} else {
			if aDepth != bDepth {
				return aDepth > bDepth
			}
		}
		return aSortPath < bSortPath
	})
}

// sortPath returns the effective path for sorting purposes.
// For external actions, this is parentPath/target (one level deeper).
func sortPath(a Action) string {
	if a.External != nil {
		return a.External.ParentPath + "/" + a.External.Target
	}
	return a.Path
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/plan/ -run TestSortWith -v`
预期：PASS

- [ ] **步骤 5：运行所有 plan 包测试**

运行：`go test ./internal/plan/ -v`
预期：全部 PASS

- [ ] **步骤 6：Commit**

```bash
git add internal/plan/sort.go internal/plan/sort_test.go
git commit -m "feat(plan): support external action sorting with effective path"
```

---

### 任务 7：svn 包 — --ignore-externals 和新命令

**文件：**
- 修改：`internal/svn/commands.go`
- 新建：`internal/svn/externals_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// internal/svn/externals_test.go
package svn

import (
	"context"
	"testing"
)

func TestGetExternalsParsing(t *testing.T) {
	// Mixed format: both old (target URL) and new (-rN URL target) formats
	output := "lib/utils svn://server/repo/trunk/utils\n-r42 svn://server/repo/trunk/proto lib/proto\n"
	extDefs, err := ParseExternalsOutput(output)
	if err != nil {
		t.Fatalf("ParseExternalsOutput: %v", err)
	}
	if len(extDefs) != 2 {
		t.Fatalf("expected 2 externals, got %d", len(extDefs))
	}
	if extDefs["lib/utils"].URL != "svn://server/repo/trunk/utils" {
		t.Errorf("lib/utils URL = %q", extDefs["lib/utils"].URL)
	}
	if extDefs["lib/utils"].Revision != "" {
		t.Errorf("lib/utils Revision = %q, want empty", extDefs["lib/utils"].Revision)
	}
	if extDefs["lib/proto"].URL != "svn://server/repo/trunk/proto" {
		t.Errorf("lib/proto URL = %q", extDefs["lib/proto"].URL)
	}
	if extDefs["lib/proto"].Revision != "42" {
		t.Errorf("lib/proto Revision = %q, want 42", extDefs["lib/proto"].Revision)
	}
}

func TestGetExternalsParsingOldFormat(t *testing.T) {
	// Old format (SVN 1.4): target [-rN] URL
	// e.g. lib/utils svn://server/repo/trunk/utils
	output := "lib/utils svn://server/repo/trunk/utils\nlib/proto -r42 svn://server/repo/trunk/proto\n"
	extDefs, err := ParseExternalsOutput(output)
	if err != nil {
		t.Fatalf("ParseExternalsOutput: %v", err)
	}
	if extDefs["lib/utils"].URL != "svn://server/repo/trunk/utils" {
		t.Errorf("lib/utils URL = %q", extDefs["lib/utils"].URL)
	}
	if extDefs["lib/proto"].Revision != "42" {
		t.Errorf("lib/proto Revision = %q, want 42", extDefs["lib/proto"].Revision)
	}
}

func TestGetExternalsParsingNewFormat(t *testing.T) {
	// New format (SVN 1.5+): [-rN] URL target
	output := "svn://server/repo/trunk/utils lib/utils\n-r42 svn://server/repo/trunk/proto lib/proto\n"
	extDefs, err := ParseExternalsOutput(output)
	if err != nil {
		t.Fatalf("ParseExternalsOutput: %v", err)
	}
	if extDefs["lib/utils"].URL != "svn://server/repo/trunk/utils" {
		t.Errorf("lib/utils URL = %q", extDefs["lib/utils"].URL)
	}
	if extDefs["lib/proto"].Revision != "42" {
		t.Errorf("lib/proto Revision = %q, want 42", extDefs["lib/proto"].Revision)
	}
}

func TestCheckoutExternalArgs(t *testing.T) {
	fc := &FakeClient{}
	ctx := context.Background()
	err := CheckoutExternal(ctx, fc, "/workdir", "src/core", "lib", "svn://server/repo/trunk/utils", "files", "", "")
	if err != nil {
		t.Fatalf("CheckoutExternal: %v", err)
	}
	if len(fc.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(fc.Calls))
	}
	args := fc.Calls[0].Args
	// Should be: checkout --depth files --ignore-externals <url> <path>
	foundIgnore := false
	foundDepth := false
	for _, arg := range args {
		if arg == "--ignore-externals" {
			foundIgnore = true
		}
		if arg == "files" {
			foundDepth = true
		}
	}
	if !foundIgnore {
		t.Error("expected --ignore-externals in args")
	}
	if !foundDepth {
		t.Error("expected 'files' depth in args")
	}
}

func TestCheckoutIgnoresExternals(t *testing.T) {
	fc := &FakeClient{}
	ctx := context.Background()
	err := Checkout(ctx, fc, "/workdir", "svn://server/repo/trunk", "")
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	args := fc.Calls[0].Args
	found := false
	for _, arg := range args {
		if arg == "--ignore-externals" {
			found = true
		}
	}
	if !found {
		t.Error("Checkout should include --ignore-externals")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/svn/ -run TestGetExternals|TestCheckoutExternal|TestCheckoutIgnore -v`
预期：编译失败（`ParseExternalsOutput`、`CheckoutExternal`、`ExternalDef` 未定义，`Checkout` 不含 `--ignore-externals`）

- [ ] **步骤 3：编写最少实现代码**

在 `internal/svn/commands.go` 中：

**首先增强 FakeClient（executor 测试需要按 args 返回不同 Stdout）**：

在 `internal/svn/fake.go` 中，给 `FakeClient` 新增 `StdoutByArgs` 字段：

```go
// StdoutByArgs: 若调用 args 包含 key 中的所有子串，则返回对应的 Stdout。
// 按列表顺序匹配，第一个匹配的生效。
StdoutByArgs []StdoutMatch

type StdoutMatch struct {
	ArgsContains []string
	Stdout       string
}
```

修改 `FakeClient.Run` 方法，在默认成功返回前检查 `StdoutByArgs`：

```go
func (f *FakeClient) Run(ctx context.Context, cwd string, args ...string) (*Result, error) {
	argsCopy := append([]string{}, args...)
	f.Calls = append(f.Calls, FakeCall{Cwd: cwd, Args: argsCopy})
	// 检查是否匹配失败规则
	for _, rule := range f.FailOn {
		if matchFailRule(rule, argsCopy) {
			return &Result{
				Args:     argsCopy,
				Stderr:   rule.Stderr,
				ExitCode: rule.ExitCode,
			}, nil
		}
	}
	// 检查 StdoutByArgs 匹配
	stdout := f.StdoutResponse
	for _, m := range f.StdoutByArgs {
		if matchFailRule(FakeFailRule{ArgsContains: m.ArgsContains}, argsCopy) {
			stdout = m.Stdout
			break
		}
	}
	// 默认成功
	return &Result{
		Args:     argsCopy,
		Stdout:   stdout,
		ExitCode: 0,
	}, nil
}
```

**然后修改 commands.go：**

1. 所有命令加 `--ignore-externals`。修改如下：

`Checkout` 函数 — 在 `args := []string{"checkout", "--depth", "empty"}` 后加 `"--ignore-externals"`：

```go
func Checkout(ctx context.Context, c Client, workdir, url string, revision string) error {
	args := []string{"checkout", "--depth", "empty", "--ignore-externals"}
	if revision != "" {
		args = append(args, "-r", revision)
	}
	args = append(args, url, workdir)
	return runAndCheck(ctx, c, "", args)
}
```

`SetDepth` 函数 — 在 args 构建中加 `"--ignore-externals"`：

```go
func SetDepth(ctx context.Context, c Client, workdir, path string, depth config.Depth, revision string) error {
	args := []string{"update", "--set-depth", depth.String(), "--ignore-externals"}
	if revision != "" {
		args = append(args, "-r", revision)
	}
	args = append(args, path)
	return runAndCheck(ctx, c, workdir, args)
}
```

`UpdateRoot` 函数 — 加 `"--ignore-externals"`：

```go
func UpdateRoot(ctx context.Context, c Client, workdir, revision string) error {
	args := []string{"update", "--ignore-externals"}
	if revision != "" {
		args = append(args, "-r", revision)
	}
	return runAndCheck(ctx, c, workdir, args)
}
```

`Exclude` 函数 — 加 `"--ignore-externals"`：

```go
func Exclude(ctx context.Context, c Client, workdir, path string, revision string) error {
	args := []string{"update", "--set-depth", "exclude", "--ignore-externals"}
	if revision != "" {
		args = append(args, "-r", revision)
	}
	args = append(args, path)
	return runAndCheck(ctx, c, workdir, args)
}
```

2. 新增类型和函数：

```go
// ExternalDef describes a single svn:externals entry.
type ExternalDef struct {
	URL      string
	Revision string
}

// ParseExternalsOutput parses svn propget svn:externals output.
// Supports both formats: "[-rN] URL target" and "target [-rN] URL"
func ParseExternalsOutput(output string) (map[string]ExternalDef, error) {
	result := make(map[string]ExternalDef)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		target, def := parseExternalsLine(line)
		if target != "" {
			result[target] = def
		}
	}
	return result, nil
}

// parseExternalsLine parses a single svn:externals line.
// Format 1: [-rN] URL target
// Format 2: target [-rN] URL
func parseExternalsLine(line string) (string, ExternalDef) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", ExternalDef{}
	}

	var revision string
	// Extract -rN if present
	remaining := make([]string, 0, len(fields))
	for i := 0; i < len(fields); i++ {
		if strings.HasPrefix(fields[i], "-r") && len(fields[i]) > 2 {
			revision = fields[i][2:]
		} else {
			remaining = append(remaining, fields[i])
		}
	}

	if len(remaining) < 2 {
		return "", ExternalDef{}
	}

	// Determine format: if last field looks like a target (no ://), it's format 1
	// Otherwise it's format 2
	last := remaining[len(remaining)-1]
	first := remaining[0]

	if strings.Contains(last, "://") {
		// Format 2: target [-rN] URL
		return first, ExternalDef{URL: last, Revision: revision}
	}
	// Format 1: [-rN] URL target
	return last, ExternalDef{URL: first, Revision: revision}
}

// GetExternals reads svn:externals property from a working copy path.
func GetExternals(ctx context.Context, c Client, workdir, path string) (map[string]ExternalDef, error) {
	result, err := c.Run(ctx, workdir, "propget", "svn:externals", path)
	if err != nil {
		return nil, fmt.Errorf("svn propget svn:externals %s: %w", path, err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("svn propget svn:externals %s: exit %d: %s", path, result.ExitCode, result.Stderr)
	}
	return ParseExternalsOutput(result.Stdout)
}

// CheckoutExternal checks out an external URL into the working copy.
func CheckoutExternal(ctx context.Context, c Client, workdir, parentPath, target, url string, depth string, extRevision, cliRevision string) error {
	args := []string{"checkout", "--depth", depth, "--ignore-externals"}
	// Use externals-defined revision if present, otherwise CLI revision
	rev := extRevision
	if rev == "" {
		rev = cliRevision
	}
	if rev != "" {
		args = append(args, "-r", rev)
	}
	args = append(args, url, filepath.Join(workdir, parentPath, target))
	return runAndCheck(ctx, c, "", args)
}
```

注意需要在文件头部确认 `"path/filepath"` 已导入（当前已有）。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/svn/ -run "TestGetExternals|TestCheckoutExternal|TestCheckoutIgnore" -v`
预期：PASS

- [ ] **步骤 5：运行全量编译**

运行：`go build ./...`
预期：编译通过

- [ ] **步骤 6：Commit**

```bash
git add internal/svn/commands.go internal/svn/externals_test.go
git commit -m "feat(svn): add --ignore-externals, GetExternals, CheckoutExternal"
```

---

### 任务 8：executor 包 — external 动作执行

**文件：**
- 修改：`internal/executor/executor.go`
- 新建：`internal/executor/executor_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// internal/executor/executor_test.go
package executor

import (
	"context"
	"testing"

	"github.com/hongy3025/sparsesvn/internal/config"
	"github.com/hongy3025/sparsesvn/internal/plan"
	"github.com/hongy3025/sparsesvn/internal/svn"
)

func TestApplyWithExternalAdd(t *testing.T) {
	dir := t.TempDir()
	yaml := "url: svn://server/repo/trunk\npaths:\n  - path: src\n    depth: infinity\n    externals:\n      - target: lib\n        depth: files\n"
	configPath := dir + "/sparsesvn.yaml"
	os.WriteFile(configPath, []byte(yaml), 0644)

	// Create a fake working copy
	os.MkdirAll(dir+"/.svn", 0755)

	fc := &svn.FakeClient{}
	opts := Options{
		ConfigPath: configPath,
		Workdir:    dir,
		Client:     fc,
	}
	result := Apply(context.Background(), opts)
	if result.Err != nil {
		t.Fatalf("Apply: %v", result.Err)
	}
	// Should have executed: SetDepth for src, then external checkout for lib
	foundExternal := false
	for _, call := range fc.Calls {
		for _, arg := range call.Args {
			if arg == "propget" {
				foundExternal = true
			}
		}
	}
	if !foundExternal {
		t.Error("expected propget call for externals")
	}
}

func TestApplyExternalExclude(t *testing.T) {
	// Test: external present in current state but not in desired -> should be excluded
	dir := t.TempDir()
	yaml := "url: svn://server/repo/trunk\npaths:\n  - path: src\n    depth: infinity\n"
	configPath := dir + "/sparsesvn.yaml"
	os.WriteFile(configPath, []byte(yaml), 0644)
	os.MkdirAll(dir+"/.svn", 0755)

	// Write a state file with an external
	stateYaml := "# sparsesvn state file - DO NOT EDIT MANUALLY\n" +
		"version: 2\n" +
		"config_hash: \"\"\n" +
		"url: \"svn://server/repo/trunk\"\n" +
		"applied_at: 2026-06-11T10:00:00Z\n" +
		"paths:\n" +
		"  - path: src\n" +
		"    depth: infinity\n" +
		"    externals:\n" +
		"      - target: lib\n" +
		"        depth: files\n"
	os.MkdirAll(dir+"/.svn", 0755)
	os.WriteFile(dir+"/.svn/sparsesvn.state.yaml", []byte(stateYaml), 0644)

	fc := &svn.FakeClient{}
	opts := Options{
		ConfigPath: configPath,
		Workdir:    dir,
		Client:     fc,
	}
	result := Apply(context.Background(), opts)
	if result.Err != nil {
		t.Fatalf("Apply: %v", result.Err)
	}
	// Should have an exclude action for the external
	foundExclude := false
	for _, a := range result.Plan {
		if a.External != nil && a.External.Target == "lib" && a.Kind == plan.ActionExclude {
			foundExclude = true
		}
	}
	if !foundExclude {
		t.Errorf("expected EXCLUDE external action for lib, plan: %v", result.Plan)
	}
}
```

需要在 test 文件头部导入 `"os"`。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/executor/ -run TestApplyWithExternal -v`
预期：FAIL（executor 未使用 `DiffWithExternals`，未处理 external 动作）

- [ ] **步骤 3：编写最少实现代码**

修改 `internal/executor/executor.go`：

1. 修改 Step 7（第 85-92 行），将 `desired` 和 `current` 构建改为使用 `expandResult`，并构建 externals 映射：

将：
```go
	// Step 7: expand + build current map
	desired := plan.Expand(cfg)
	current := make(map[string]config.Depth)
	if exists {
		for _, p := range st.Paths {
			current[p.Path] = p.Depth
		}
	}
```

改为：
```go
	// Step 7: expand + build current map
	expandResult := plan.Expand(cfg)
	desired := expandResult.Paths
	current := make(map[string]config.Depth)
	currentExt := make(map[string][]plan.ExternalSpec)
	if exists {
		for _, p := range st.Paths {
			current[p.Path] = p.Depth
			var exts []plan.ExternalSpec
			for _, e := range p.Externals {
				exts = append(exts, plan.ExternalSpec{Target: e.Target, Depth: e.Depth})
			}
			if exts == nil {
				exts = []plan.ExternalSpec{}
			}
			currentExt[p.Path] = exts
		}
	}
```

2. 修改 Step 8（第 94-97 行），将 `plan.Diff` 改为 `plan.DiffWithExternals`：

将：
```go
	// Step 8: diff + sort
	actions := plan.Diff(desired, current)
	plan.Sort(actions)
	r.Plan = actions
```

改为：
```go
	// Step 8: diff + sort
	actions := plan.DiffWithExternals(desired, current, expandResult.Externals, currentExt)
	plan.Sort(actions)
	r.Plan = actions
```

3. 修改 Step 12 的动作执行循环（第 129-160 行），增加 external 动作处理。将执行循环替换为：

```go
	// Step 12: execute actions
	executedCount := 0
	for i := range actions {
		a := &actions[i]
		var execErr error
		if a.External != nil {
			execErr = executeExternalAction(ctx, opts, a)
		} else {
			switch a.Kind {
			case plan.ActionAdd, plan.ActionUpgrade, plan.ActionDowngrade:
				execErr = svn.SetDepth(ctx, opts.Client, opts.Workdir, a.Path, a.ToDepth, opts.Revision)
			case plan.ActionExclude:
				execErr = svn.Exclude(ctx, opts.Client, opts.Workdir, a.Path, opts.Revision)
			}
		}
		if execErr != nil {
			r.FailedAction = a
			r.Err = fmt.Errorf("%w: action %s %s: %w", ErrSvnFailed, a.Kind, actionLabel(a), execErr)
			r.ExecutedCount = executedCount
			// Write half-state
			halfState := buildStateFromMaps("", finalURL, current, currentExt)
			if saveErr := state.Save(opts.Workdir, halfState); saveErr != nil {
				r.Err = fmt.Errorf("%w; save state: %v", r.Err, saveErr)
			}
			r.StateAfter = halfState
			return r
		}
		executedCount++
		// Apply change to current map
		if a.External != nil {
			applyExternalActionToCurrent(a, currentExt)
		} else {
			switch a.Kind {
			case plan.ActionAdd, plan.ActionUpgrade, plan.ActionDowngrade:
				current[a.Path] = a.ToDepth
			case plan.ActionExclude:
				delete(current, a.Path)
				delete(currentExt, a.Path)
			}
		}
	}
```

4. 新增辅助函数（在 `buildState` 函数之后）：

```go
// actionLabel returns a human-readable label for the action.
func actionLabel(a *plan.Action) string {
	if a.External != nil {
		return a.External.ParentPath + "/" + a.External.Target
	}
	return a.Path
}

// executeExternalAction executes an external action.
func executeExternalAction(ctx context.Context, opts Options, a *plan.Action) error {
	switch a.Kind {
	case plan.ActionAdd:
		// Get svn:externals definition to find source URL
		extDefs, err := svn.GetExternals(ctx, opts.Client, opts.Workdir, a.External.ParentPath)
		if err != nil {
			return fmt.Errorf("get externals for %s: %w", a.External.ParentPath, err)
		}
		extDef, ok := extDefs[a.External.Target]
		if !ok {
			return fmt.Errorf("external %q not found in svn:externals of %s", a.External.Target, a.External.ParentPath)
		}
		return svn.CheckoutExternal(ctx, opts.Client, opts.Workdir, a.External.ParentPath, a.External.Target, extDef.URL, a.ToDepth.String(), extDef.Revision, opts.Revision)
	case plan.ActionUpgrade, plan.ActionDowngrade:
		return svn.SetDepth(ctx, opts.Client, opts.Workdir, a.External.ParentPath+"/"+a.External.Target, a.ToDepth, opts.Revision)
	case plan.ActionExclude:
		return svn.Exclude(ctx, opts.Client, opts.Workdir, a.External.ParentPath+"/"+a.External.Target, opts.Revision)
	default:
		return fmt.Errorf("unknown external action kind: %d", a.Kind)
	}
}

// applyExternalActionToCurrent updates currentExt based on the executed action.
func applyExternalActionToCurrent(a *plan.Action, currentExt map[string][]plan.ExternalSpec) {
	parentPath := a.External.ParentPath
	exts := currentExt[parentPath]
	switch a.Kind {
	case plan.ActionAdd, plan.ActionUpgrade, plan.ActionDowngrade:
		found := false
		for i := range exts {
			if exts[i].Target == a.External.Target {
				exts[i].Depth = a.ToDepth
				found = true
				break
			}
		}
		if !found {
			exts = append(exts, plan.ExternalSpec{Target: a.External.Target, Depth: a.ToDepth})
		}
		currentExt[parentPath] = exts
	case plan.ActionExclude:
		filtered := make([]plan.ExternalSpec, 0, len(exts))
		for _, e := range exts {
			if e.Target != a.External.Target {
				filtered = append(filtered, e)
			}
		}
		currentExt[parentPath] = filtered
	}
}
```

5. 修改 `buildState` 函数以支持 externals，并新增 `buildStateFromMaps`：

将原 `buildState` 改为：

```go
func buildState(configHash, url string, current map[string]config.Depth) *state.State {
	return buildStateFromMaps(configHash, url, current, nil)
}

func buildStateFromMaps(configHash, url string, current map[string]config.Depth, currentExt map[string][]plan.ExternalSpec) *state.State {
	paths := make([]state.PathEntry, 0, len(current))
	for p, d := range current {
		pe := state.PathEntry{Path: p, Depth: d}
		if currentExt != nil {
			for _, ext := range currentExt[p] {
				pe.Externals = append(pe.Externals, state.ExternalEntry{Target: ext.Target, Depth: ext.Depth})
			}
		}
		paths = append(paths, pe)
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i].Path < paths[j].Path })
	return &state.State{
		Version:    state.StateVersion,
		ConfigHash: configHash,
		URL:        url,
		AppliedAt:  time.Now().UTC(),
		Paths:      paths,
	}
}
```

6. 同样更新 half-state 写回代码（第 145 行附近），将 `buildState("", finalURL, current)` 改为 `buildStateFromMaps("", finalURL, current, currentExt)`。

7. 更新成功路径的 state 写入（第 165 行附近），将 `buildState(configHash, finalURL, current)` 改为 `buildStateFromMaps(configHash, finalURL, current, currentExt)`。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/executor/ -run TestApplyWithExternal -v`
预期：PASS

- [ ] **步骤 5：运行全量编译和测试**

运行：`go build ./... && go test ./internal/... -v`
预期：编译通过，所有测试通过

- [ ] **步骤 6：Commit**

```bash
git add internal/executor/executor.go internal/executor/executor_test.go
git commit -m "feat(executor): execute external actions with propget + checkout"
```

---

### 任务 9：cli 包 — output 格式支持 external 动作

**文件：**
- 修改：`internal/cli/output.go`

- [ ] **步骤 1：编写失败的测试**

（此任务修改较小，不单独新增测试文件，在后续集成测试中验证）

- [ ] **步骤 2：修改 `FormatPlan` 支持 external 动作显示**

在 `internal/cli/output.go` 的 `FormatPlan` 函数中，修改动作输出部分（第 63-74 行）。在每个 `case` 分支中增加 external 判断：

将循环体替换为：

```go
	for _, a := range actions {
		marker := kindMarker(a.Kind)
		label := kindLabel(a.Kind)
		displayPath := a.Path
		if a.External != nil {
			displayPath = a.External.ParentPath + "/" + a.External.Target
		}
		switch a.Kind {
		case plan.ActionAdd:
			fmt.Fprintf(tw, "%s %s\t%s\t-> %s\n", marker, label, displayPath, a.ToDepth)
		case plan.ActionUpgrade, plan.ActionDowngrade:
			fmt.Fprintf(tw, "%s %s\t%s\t%s -> %s\n", marker, label, displayPath, a.FromDepth, a.ToDepth)
		case plan.ActionExclude:
			fmt.Fprintf(tw, "%s %s\t%s\t%s\n", marker, label, displayPath, a.FromDepth)
		}
	}
```

- [ ] **步骤 3：修改 `BuildPlanJSON` 支持 external 动作**

在 `ActionJSON` 结构体中新增 `Target` 字段：

```go
type ActionJSON struct {
	Kind      string `json:"kind"`
	Path      string `json:"path"`
	Target    string `json:"target,omitempty"`
	FromDepth string `json:"from_depth,omitempty"`
	ToDepth   string `json:"to_depth,omitempty"`
}
```

在 `BuildPlanJSON` 中构建 `aj` 时，添加 external 信息：

在 `aj := ActionJSON{...}` 之后加：

```go
		if a.External != nil {
			aj.Target = a.External.Target
			aj.Path = a.External.ParentPath + "/" + a.External.Target
		}
```

- [ ] **步骤 4：运行编译和现有测试**

运行：`go build ./...`
预期：编译通过

- [ ] **步骤 5：Commit**

```bash
git add internal/cli/output.go
git commit -m "feat(cli): display external actions in plan output"
```

---

### 任务 10：全量编译验证和边界场景测试

**文件：**
- 无新增文件

- [ ] **步骤 1：全量编译**

运行：`go build ./...`
预期：编译通过

- [ ] **步骤 2：运行所有单元测试**

运行：`go test ./internal/... -v`
预期：全部 PASS

- [ ] **步骤 3：手动验证 — 构建二进制**

运行：`go build -o sparsesvn.exe ./cmd/sparsesvn`
预期：成功生成 `sparsesvn.exe`

- [ ] **步骤 4：手动验证 — validate 子命令**

创建一个带 externals 的测试配置文件，运行 `./sparsesvn.exe validate -f <path>`，确认：
- 合法配置通过
- `depth: empty` + externals 报错
- target 含 `/` 报错

- [ ] **步骤 5：Commit（如有修复）**

```bash
git add -A && git commit -m "fix: address issues found in full validation"
```

---

### 任务 11：最终清理和文档

**文件：**
- 修改：`docs/superpowers/specs/2026-06-11-externals-sparse-depth-design.md`（状态改为已实现）

- [ ] **步骤 1：更新设计文档状态**

将设计文档中的 `状态：草案（待实现）` 改为 `状态：已实现`

- [ ] **步骤 2：运行最终全量测试**

运行：`go test ./... -v`
预期：全部 PASS

- [ ] **步骤 3：Commit**

```bash
git add docs/superpowers/specs/2026-06-11-externals-sparse-depth-design.md
git commit -m "docs: mark externals design as implemented"
```
