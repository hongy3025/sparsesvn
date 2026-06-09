package svn

import (
	"context"
	"fmt"
	"strings"
)

// FakeClient 是 Client 的内存实现，用于单元测试 executor 和 commands
type FakeClient struct {
	// Calls 按调用顺序记录所有 Run 调用
	Calls []FakeCall
	// VersionResponse：Version() 返回这个三元组
	VersionResponse struct {
		Major, Minor, Patch int
		Err                 error
	}
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

func (f *FakeClient) Run(ctx context.Context, cwd string, args ...string) (*Result, error) {
	f.Calls = append(f.Calls, FakeCall{Cwd: cwd, Args: args})
	// 检查是否匹配失败规则
	for _, rule := range f.FailOn {
		if matchFailRule(rule, args) {
			return &Result{
				Args:     args,
				Stderr:   rule.Stderr,
				ExitCode: rule.ExitCode,
			}, fmt.Errorf("fake failure: %s", rule.Stderr)
		}
	}
	// 默认成功
	return &Result{
		Args:     args,
		ExitCode: 0,
	}, nil
}

func matchFailRule(rule FakeFailRule, args []string) bool {
	for _, substr := range rule.ArgsContains {
		found := false
		for _, arg := range args {
			if strings.Contains(arg, substr) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (f *FakeClient) Version(ctx context.Context) (major, minor, patch int, err error) {
	return f.VersionResponse.Major, f.VersionResponse.Minor, f.VersionResponse.Patch, f.VersionResponse.Err
}

// Reset 清空 Calls，便于测试间复用
func (f *FakeClient) Reset() {
	f.Calls = nil
}