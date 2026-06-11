package svn

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hongy3025/sparsesvn/internal/config"
)

func TestCheckout_BuildsArgs(t *testing.T) {
	f := &FakeClient{}
	ctx := context.Background()
	err := Checkout(ctx, f, "/tmp/w", "svn://x", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(f.Calls))
	}
	call := f.Calls[0]
	want := []string{"checkout", "--depth", "empty", "--ignore-externals", "svn://x", "/tmp/w"}
	if len(call.Args) != len(want) {
		t.Fatalf("args len = %d, want %d; got %v", len(call.Args), len(want), call.Args)
	}
	for i, w := range want {
		if call.Args[i] != w {
			t.Errorf("args[%d] = %q, want %q", i, call.Args[i], w)
		}
	}
}

func TestCheckout_WithRevision(t *testing.T) {
	f := &FakeClient{}
	ctx := context.Background()
	err := Checkout(ctx, f, "/tmp/w", "svn://x", "100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := f.Calls[0]
	want := []string{"checkout", "--depth", "empty", "--ignore-externals", "-r", "100", "svn://x", "/tmp/w"}
	if len(call.Args) != len(want) {
		t.Fatalf("args len = %d, want %d; got %v", len(call.Args), len(want), call.Args)
	}
	for i, w := range want {
		if call.Args[i] != w {
			t.Errorf("args[%d] = %q, want %q", i, call.Args[i], w)
		}
	}
}

func TestSetDepth_Empty(t *testing.T) {
	f := &FakeClient{}
	ctx := context.Background()
	err := SetDepth(ctx, f, "/workdir", "trunk/src", config.DepthEmpty, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := f.Calls[0]
	want := []string{"update", "--set-depth", "empty", "--ignore-externals", "trunk/src"}
	assertArgs(t, call.Args, want)
	if call.Cwd != "/workdir" {
		t.Errorf("cwd = %q, want %q", call.Cwd, "/workdir")
	}
}

func TestSetDepth_Files(t *testing.T) {
	f := &FakeClient{}
	ctx := context.Background()
	err := SetDepth(ctx, f, "/workdir", "trunk/src", config.DepthFiles, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := f.Calls[0]
	want := []string{"update", "--set-depth", "files", "--ignore-externals", "trunk/src"}
	assertArgs(t, call.Args, want)
}

func TestSetDepth_Infinity(t *testing.T) {
	f := &FakeClient{}
	ctx := context.Background()
	err := SetDepth(ctx, f, "/workdir", "trunk/src", config.DepthInfinity, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := f.Calls[0]
	want := []string{"update", "--set-depth", "infinity", "--ignore-externals", "trunk/src"}
	assertArgs(t, call.Args, want)
}

func TestSetDepth_WithRevision(t *testing.T) {
	f := &FakeClient{}
	ctx := context.Background()
	err := SetDepth(ctx, f, "/workdir", "trunk/src", config.DepthEmpty, "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := f.Calls[0]
	want := []string{"update", "--set-depth", "empty", "--ignore-externals", "-r", "42", "trunk/src"}
	assertArgs(t, call.Args, want)
}

func TestExclude_BuildsArgs(t *testing.T) {
	f := &FakeClient{}
	ctx := context.Background()
	err := Exclude(ctx, f, "/workdir", "trunk/old", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := f.Calls[0]
	want := []string{"update", "--set-depth", "exclude", "--ignore-externals", "trunk/old"}
	assertArgs(t, call.Args, want)
	if call.Cwd != "/workdir" {
		t.Errorf("cwd = %q, want %q", call.Cwd, "/workdir")
	}
}

func TestUpdateRoot_BuildsArgs(t *testing.T) {
	f := &FakeClient{}
	ctx := context.Background()
	err := UpdateRoot(ctx, f, "/workdir", "500")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := f.Calls[0]
	want := []string{"update", "--ignore-externals", "-r", "500"}
	assertArgs(t, call.Args, want)
	if call.Cwd != "/workdir" {
		t.Errorf("cwd = %q, want %q", call.Cwd, "/workdir")
	}
}

func TestIsWorkingCopy_True(t *testing.T) {
	dir := t.TempDir()
	svnDir := filepath.Join(dir, ".svn")
	if err := os.Mkdir(svnDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if !IsWorkingCopy(dir) {
		t.Errorf("expected true for directory with .svn/")
	}
}

func TestIsWorkingCopy_False(t *testing.T) {
	dir := t.TempDir()
	if IsWorkingCopy(dir) {
		t.Errorf("expected false for directory without .svn/")
	}
}

func TestGetWorkingCopyURL(t *testing.T) {
	f := &FakeClient{
		StdoutResponse: "svn://svn.example.com/repo/trunk\n",
	}
	ctx := context.Background()

	got, err := GetWorkingCopyURL(ctx, f, "/tmp/w")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "svn://svn.example.com/repo/trunk" {
		t.Errorf("GetWorkingCopyURL() = %q, want %q", got, "svn://svn.example.com/repo/trunk")
	}
	// 验证调用参数
	if len(f.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(f.Calls))
	}
	call := f.Calls[0]
	want := []string{"info", "--show-item", "url"}
	assertArgs(t, call.Args, want)
	if call.Cwd != "/tmp/w" {
		t.Errorf("cwd = %q, want %q", call.Cwd, "/tmp/w")
	}
}

func TestGetWorkingCopyURL_NotWorkingCopy(t *testing.T) {
	f := &FakeClient{
		FailOn: []FakeFailRule{
			{ArgsContains: []string{"info", "--show-item", "url"}, Stderr: "E155007", ExitCode: 1},
		},
	}
	ctx := context.Background()

	_, err := GetWorkingCopyURL(ctx, f, "/tmp/w")
	if err == nil {
		t.Fatal("expected error for non-working copy, got nil")
	}
}

func TestSetDepth_FailingExitPropagatesError(t *testing.T) {
	f := &FakeClient{
		FailOn: []FakeFailRule{
			{ArgsContains: []string{"--set-depth", "exclude"}, Stderr: "cannot exclude root", ExitCode: 1},
		},
	}
	ctx := context.Background()
	err := Exclude(ctx, f, "/workdir", "trunk/bad", "")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got := err.Error(); !contains(got, "exit 1") {
		t.Errorf("error = %q, want it to contain %q", got, "exit 1")
	}
}

func assertArgs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("args len = %d, want %d; got %v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("args[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
