package svn

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Result 包含一次 svn 命令的执行结果
type Result struct {
	Args     []string
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// Client 是与 svn 二进制交互的唯一接口
type Client interface {
	// Run 执行 svn <args...>，cwd 为工作目录
	Run(ctx context.Context, cwd string, args ...string) (*Result, error)
	// Version 返回 svn 版本号（major, minor, patch），用于能力检测
	Version(ctx context.Context) (major, minor, patch int, err error)
}

// NewExecClient 返回一个调用 PATH 中 `svn` 二进制的实现
func NewExecClient() Client {
	return &execClient{}
}

// execClient 使用 os/exec 调用真实的 svn 二进制
type execClient struct{}

func (c *execClient) Run(ctx context.Context, cwd string, args ...string) (*Result, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, "svn", args...)
	cmd.Dir = cwd

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	result := &Result{
		Args:     args,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, nil // 命令执行了但退出码非零
		}
		// 进程未启动等其他错误
		return nil, err
	}
	result.ExitCode = 0
	return result, nil
}

func (c *execClient) Version(ctx context.Context) (major, minor, patch int, err error) {
	result, err := c.Run(ctx, "", "--version", "--quiet")
	if err != nil {
		return 0, 0, 0, err
	}
	if result.ExitCode != 0 {
		return 0, 0, 0, fmt.Errorf("svn --version --quiet failed with exit code %d: %s", result.ExitCode, result.Stderr)
	}
	return parseVersion(result.Stdout)
}