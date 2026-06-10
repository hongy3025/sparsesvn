package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateWorkdir_DefaultWorkdir_NoSvn(t *testing.T) {
	// 在没有 .svn 的目录，使用默认 -C
	tmpDir := t.TempDir()
	original, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(original)

	gf := &GlobalFlags{
		Workdir:         ".",
		WorkdirExplicit: false,
		ConfigFile:      filepath.Join(tmpDir, "sparsesvn.yaml"),
	}

	// 创建一个有效的配置文件
	os.WriteFile(gf.ConfigFile, []byte("url: svn://example.com/repo\npaths:\n  - path: src\n    depth: infinity\n"), 0644)

	err := validateAndDisplayContext(gf, os.Stdout)
	if err == nil {
		t.Fatal("expected error for default workdir without .svn, got nil")
	}
}

func TestValidateWorkdir_ExplicitWorkdir_NoSvn(t *testing.T) {
	// 显式指定 -C 到没有 .svn 的目录，应该允许
	tmpDir := t.TempDir()

	gf := &GlobalFlags{
		Workdir:         tmpDir,
		WorkdirExplicit: true,
		ConfigFile:      filepath.Join(tmpDir, "sparsesvn.yaml"),
	}

	// 创建一个有效的配置文件
	os.WriteFile(gf.ConfigFile, []byte("url: svn://example.com/repo\npaths:\n  - path: src\n    depth: infinity\n"), 0644)

	err := validateAndDisplayContext(gf, os.Stdout)
	if err != nil {
		t.Fatalf("unexpected error for explicit workdir without .svn: %v", err)
	}
	if gf.ResolvedURL != "svn://example.com/repo" {
		t.Errorf("ResolvedURL = %q, want %q", gf.ResolvedURL, "svn://example.com/repo")
	}
}

func TestDisplayContext(t *testing.T) {
	buf := new(bytes.Buffer)
	displayContext(buf, "/tmp/project", "svn://example.com/repo", "./config.yaml")

	expected := "Working directory: /tmp/project\nRepository URL:    svn://example.com/repo\nConfig file:       ./config.yaml\n"
	if buf.String() != expected {
		t.Errorf("displayContext() output mismatch:\ngot:  %q\nwant: %q", buf.String(), expected)
	}
}
