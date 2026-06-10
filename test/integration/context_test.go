//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextDisplay_NormalExecution(t *testing.T) {
	// 测试在正常工作副本目录执行时显示上下文
	repo := CreateTestRepo(t)
	workdir := t.TempDir()

	// 创建配置文件
	cfgPath := filepath.Join(workdir, "sparsesvn.yaml")
	cfgContent := "url: " + repo.URL + "\npaths:\n  - path: trunk/src\n    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// 先执行 apply 创建工作副本
	stdout, stderr, code := RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("apply failed: code=%d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	// 执行 plan 命令
	stdout, stderr, code = RunCLI(t, []string{"plan", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("plan failed: code=%d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	// 验证上下文信息被显示
	if !strings.Contains(stderr, "Working directory:") {
		t.Error("expected 'Working directory:' in stderr")
	}
	if !strings.Contains(stderr, "Repository URL:") {
		t.Error("expected 'Repository URL:' in stderr")
	}
	if !strings.Contains(stderr, "Config file:") {
		t.Error("expected 'Config file:' in stderr")
	}
}

func TestContextDisplay_QuietMode(t *testing.T) {
	// 测试 quiet 模式不显示上下文
	repo := CreateTestRepo(t)
	workdir := t.TempDir()

	// 创建配置文件
	cfgPath := filepath.Join(workdir, "sparsesvn.yaml")
	cfgContent := "url: " + repo.URL + "\npaths:\n  - path: trunk/src\n    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// 执行 plan 命令，使用 quiet 模式
	stdout, stderr, code := RunCLI(t, []string{"plan", "-f", cfgPath, "-C", workdir, "-q"}, "")
	if code != 0 {
		t.Fatalf("plan failed: code=%d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	// 验证上下文信息不被显示
	if strings.Contains(stderr, "Working directory:") {
		t.Error("unexpected 'Working directory:' in stderr for quiet mode")
	}
}

func TestContextDisplay_DefaultWorkdir_NoSvn(t *testing.T) {
	// 测试在非 SVN 工作副本目录运行（未指定 -C）
	original, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(original)

	// 创建配置文件
	cfgPath := filepath.Join(tmpDir, "sparsesvn.yaml")
	cfgContent := "url: svn://example.com/repo\npaths:\n  - path: src\n    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// 执行 plan 命令（使用默认 workdir）
	_, stderr, code := RunCLI(t, []string{"plan", "-f", cfgPath}, "")

	// 应该报错退出
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d\nstderr: %s", code, stderr)
	}

	// 验证错误信息
	if !strings.Contains(stderr, "not an SVN working copy") {
		t.Errorf("expected error about not being SVN working copy, got: %s", stderr)
	}
}

func TestContextDisplay_ExplicitWorkdir_NoSvn(t *testing.T) {
	// 测试显式指定 -C 到非 SVN 工作副本目录（首次 checkout）
	repo := CreateTestRepo(t)
	workdir := t.TempDir()

	// 创建配置文件
	cfgPath := filepath.Join(workdir, "sparsesvn.yaml")
	cfgContent := "url: " + repo.URL + "\npaths:\n  - path: trunk/src\n    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// 执行 plan 命令，显式指定 -C
	stdout, stderr, code := RunCLI(t, []string{"plan", "-f", cfgPath, "-C", workdir}, "")

	// 应该成功（首次 checkout 场景）
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
}
