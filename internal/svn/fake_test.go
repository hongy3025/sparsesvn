package svn

import (
	"context"
	"testing"
)

func TestFakeClient_RecordsCalls(t *testing.T) {
	f := &FakeClient{}
	ctx := context.Background()
	// 三次调用
	f.Run(ctx, "/cwd1", "update", "src")
	f.Run(ctx, "/cwd2", "commit", "-m", "msg")
	f.Run(ctx, "/cwd3", "status")
	if len(f.Calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(f.Calls))
	}
	// 检查参数
	if f.Calls[0].Cwd != "/cwd1" || f.Calls[0].Args[0] != "update" {
		t.Errorf("call 0: unexpected %+v", f.Calls[0])
	}
	if f.Calls[1].Cwd != "/cwd2" || f.Calls[1].Args[0] != "commit" {
		t.Errorf("call 1: unexpected %+v", f.Calls[1])
	}
	if f.Calls[2].Cwd != "/cwd3" || f.Calls[2].Args[0] != "status" {
		t.Errorf("call 2: unexpected %+v", f.Calls[2])
	}
}

func TestFakeClient_DefaultsToSuccess(t *testing.T) {
	f := &FakeClient{}
	ctx := context.Background()
	result, err := f.Run(ctx, "", "any")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestFakeClient_FailOnMatches(t *testing.T) {
	f := &FakeClient{
		FailOn: []FakeFailRule{
			{ArgsContains: []string{"update", "src/nonexistent"}, Stderr: "path not found", ExitCode: 1},
		},
	}
	ctx := context.Background()
	// 匹配
	result, err := f.Run(ctx, "", "update", "src/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", result.ExitCode)
	}
	if result.Stderr != "path not found" {
		t.Errorf("expected stderr 'path not found', got %q", result.Stderr)
	}
	// 不匹配
	result, err = f.Run(ctx, "", "update", "src/other")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestFakeClient_VersionResponse(t *testing.T) {
	f := &FakeClient{}
	f.VersionResponse.Major = 1
	f.VersionResponse.Minor = 14
	f.VersionResponse.Patch = 2
	f.VersionResponse.Err = nil
	ctx := context.Background()
	major, minor, patch, err := f.Version(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if major != 1 || minor != 14 || patch != 2 {
		t.Errorf("expected (1,14,2), got (%d,%d,%d)", major, minor, patch)
	}
}