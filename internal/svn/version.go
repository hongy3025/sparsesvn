package svn

import (
	"fmt"
	"regexp"
	"strconv"
)

var versionRegexp = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)

// parseVersion 从 svn --version --quiet 的输出中解析版本号
func parseVersion(output string) (major, minor, patch int, err error) {
	matches := versionRegexp.FindStringSubmatch(output)
	if matches == nil {
		return 0, 0, 0, fmt.Errorf("invalid version output: %q", output)
	}
	major, err = strconv.Atoi(matches[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major version: %w", err)
	}
	minor, err = strconv.Atoi(matches[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid minor version: %w", err)
	}
	patch, err = strconv.Atoi(matches[3])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid patch version: %w", err)
	}
	return major, minor, patch, nil
}