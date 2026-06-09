package svn

import (
	"fmt"
	"regexp"
)

var versionRegexp = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)

// parseVersion 从 svn --version --quiet 的输出中解析版本号
func parseVersion(output string) (major, minor, patch int, err error) {
	matches := versionRegexp.FindStringSubmatch(output)
	if matches == nil {
		return 0, 0, 0, fmt.Errorf("invalid version output: %q", output)
	}
	// matches[1] 是 major，matches[2] 是 minor，matches[3] 是 patch
	_, err = fmt.Sscanf(matches[1], "%d", &major)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major version: %w", err)
	}
	_, err = fmt.Sscanf(matches[2], "%d", &minor)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid minor version: %w", err)
	}
	_, err = fmt.Sscanf(matches[3], "%d", &patch)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid patch version: %w", err)
	}
	return major, minor, patch, nil
}