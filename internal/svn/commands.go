package svn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hongy3025/sparsesvn/internal/config"
)

func Checkout(ctx context.Context, c Client, workdir, url string, revision string) error {
	args := []string{"checkout", "--depth", "empty"}
	if revision != "" {
		args = append(args, "-r", revision)
	}
	args = append(args, url, workdir)
	return runAndCheck(ctx, c, "", args)
}

func SetDepth(ctx context.Context, c Client, workdir, path string, depth config.Depth, revision string) error {
	args := []string{"update", "--set-depth", depth.String()}
	if revision != "" {
		args = append(args, "-r", revision)
	}
	args = append(args, path)
	return runAndCheck(ctx, c, workdir, args)
}

func UpdateRoot(ctx context.Context, c Client, workdir, revision string) error {
	args := []string{"update"}
	if revision != "" {
		args = append(args, "-r", revision)
	}
	return runAndCheck(ctx, c, workdir, args)
}

func IsWorkingCopy(workdir string) bool {
	info, err := os.Stat(filepath.Join(workdir, ".svn"))
	return err == nil && info.IsDir()
}

func Exclude(ctx context.Context, c Client, workdir, path string, revision string) error {
	args := []string{"update", "--set-depth", "exclude"}
	if revision != "" {
		args = append(args, "-r", revision)
	}
	args = append(args, path)
	return runAndCheck(ctx, c, workdir, args)
}

// GetWorkingCopyURL 通过 svn info 获取工作副本的真实 URL
func GetWorkingCopyURL(ctx context.Context, c Client, workdir string) (string, error) {
	result, err := c.Run(ctx, workdir, "info", "--show-item", "url")
	if err != nil {
		return "", fmt.Errorf("svn info: %w", err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("not a working copy: %s", result.Stderr)
	}
	return strings.TrimSpace(result.Stdout), nil
}

func runAndCheck(ctx context.Context, c Client, cwd string, args []string) error {
	result, err := c.Run(ctx, cwd, args...)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("svn %s failed: exit %d: %s", strings.Join(args, " "), result.ExitCode, result.Stderr)
	}
	return nil
}
