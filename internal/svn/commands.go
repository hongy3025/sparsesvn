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
	args := []string{"checkout", "--depth", "empty", "--ignore-externals"}
	if revision != "" {
		args = append(args, "-r", revision)
	}
	args = append(args, url, workdir)
	return runAndCheck(ctx, c, "", args)
}

func SetDepth(ctx context.Context, c Client, workdir, path string, depth config.Depth, revision string) error {
	args := []string{"update", "--set-depth", depth.String(), "--ignore-externals"}
	if revision != "" {
		args = append(args, "-r", revision)
	}
	args = append(args, path)
	return runAndCheck(ctx, c, workdir, args)
}

func UpdateRoot(ctx context.Context, c Client, workdir, revision string) error {
	args := []string{"update", "--ignore-externals"}
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
	args := []string{"update", "--set-depth", "exclude", "--ignore-externals"}
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

// ExternalDef describes a single svn:externals entry.
type ExternalDef struct {
	URL      string
	Revision string
}

// ParseExternalsOutput parses svn propget svn:externals output.
// Supports both formats:
//   - Old (SVN 1.4): target [-rN] URL
//   - New (SVN 1.5+): [-rN] URL target
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
func parseExternalsLine(line string) (string, ExternalDef) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", ExternalDef{}
	}

	var revision string
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
