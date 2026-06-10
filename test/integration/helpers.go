//go:build integration

package integration

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// SvnRepo 表示一个本地测试用 svn 仓库
type SvnRepo struct {
	Path string // 仓库目录绝对路径
	URL  string // file:// 形式的 URL
}

var (
	binaryPath string
	binaryOnce sync.Once
	binaryErr  error
)

// RequireSvnBinary 在所有集成测试开始前检查 svn 和 svnadmin 是否可用
func RequireSvnBinary(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("svn"); err != nil {
		t.Skip("svn not found in PATH, skipping integration test")
	}
	if _, err := exec.LookPath("svnadmin"); err != nil {
		t.Skip("svnadmin not found in PATH, skipping integration test")
	}
}

// CreateTestRepo 在 t.TempDir 中创建一个 svn 仓库，预置标准目录结构
func CreateTestRepo(t *testing.T) *SvnRepo {
	t.Helper()
	RequireSvnBinary(t)

	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	importDir := filepath.Join(tmpDir, "import")

	// svnadmin create
	if out, err := exec.Command("svnadmin", "create", repoPath).CombinedOutput(); err != nil {
		t.Fatalf("svnadmin create failed: %v\n%s", err, out)
	}

	// 准备导入目录结构
	files := map[string]string{
		filepath.Join(importDir, "trunk", "src", "core", "main.c"):         "// main.c\n",
		filepath.Join(importDir, "trunk", "src", "core", "util.c"):         "// util.c\n",
		filepath.Join(importDir, "trunk", "src", "utils", "helper.c"):      "// helper.c\n",
		filepath.Join(importDir, "trunk", "docs", "readme.md"):             "# readme\n",
		filepath.Join(importDir, "trunk", "tests", "unit", "test_main.c"):  "// test_main.c\n",
		filepath.Join(importDir, "trunk", "tests", "integration", "test_api.c"): "// test_api.c\n",
	}

	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write file failed: %v", err)
		}
	}

	// 构造 file:// URL
	repoURL := filepath.ToSlash(repoPath)
	if runtime.GOOS == "windows" {
		// Windows: file:///C:/path/to/repo
		repoURL = "file:///" + repoURL
	} else {
		repoURL = "file://" + repoURL
	}

	// svn import
	cmd := exec.Command("svn", "import", importDir, repoURL, "-m", "init")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("svn import failed: %v\n%s", err, out)
	}

	return &SvnRepo{
		Path: repoPath,
		URL:  repoURL,
	}
}

// BuildBinary 用 go build 构建 sparsesvn，返回二进制路径（全局缓存）
func BuildBinary(t *testing.T) string {
	t.Helper()

	binaryOnce.Do(func() {
		tmpDir, err := os.MkdirTemp("", "sparsesvn-test-*")
		if err != nil {
			binaryErr = fmt.Errorf("create temp dir: %w", err)
			return
		}

		name := "sparsesvn"
		if runtime.GOOS == "windows" {
			name = "sparsesvn.exe"
		}
		bp := filepath.Join(tmpDir, name)

		root, err := findModuleRoot()
		if err != nil {
			binaryErr = fmt.Errorf("find module root: %w", err)
			return
		}

		cmd := exec.Command("go", "build", "-o", bp, "./cmd/sparsesvn")
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			binaryErr = fmt.Errorf("go build failed: %v\n%s", err, out)
			return
		}
		binaryPath = bp
	})

	if binaryErr != nil {
		t.Fatal(binaryErr)
	}
	return binaryPath
}

// RunCLI 构建并运行 sparsesvn 二进制，返回 stdout / stderr / exit code
func RunCLI(t *testing.T, args []string, workdir string) (stdout, stderr string, exitCode int) {
	t.Helper()
	bin := BuildBinary(t)

	cmd := exec.Command(bin, args...)
	if workdir != "" {
		cmd.Dir = workdir
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("run CLI failed: %v", err)
		}
	}

	return outBuf.String(), errBuf.String(), exitCode
}

// findModuleRoot 从当前文件向上查找 go.mod 所在目录
func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}
