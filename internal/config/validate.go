package config

import (
	"fmt"
	"strings"
)

func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if len(cfg.Paths) == 0 {
		return fmt.Errorf("paths must not be empty")
	}

	seen := make(map[string]int, len(cfg.Paths))
	for i, p := range cfg.Paths {
		if err := validatePath(p.Path); err != nil {
			return fmt.Errorf("paths[%d] %q: %w", i, p.Path, err)
		}
		if prev, ok := seen[p.Path]; ok {
			return fmt.Errorf("paths[%d] %q: duplicate of paths[%d]", i, p.Path, prev)
		}
		seen[p.Path] = i
	}
	return nil
}

func validatePath(p string) error {
	if p == "" {
		return fmt.Errorf("path must not be empty")
	}
	if strings.Contains(p, `\`) {
		return fmt.Errorf("path must not contain backslash; use forward slash")
	}
	if strings.HasPrefix(p, "/") {
		return fmt.Errorf("path must not start with '/'")
	}
	if strings.HasSuffix(p, "/") {
		return fmt.Errorf("path must not end with '/'")
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" {
			return fmt.Errorf("path must not contain empty segment ('//')")
		}
		if seg == ".." {
			return fmt.Errorf("path must not contain '..' segment")
		}
	}
	return nil
}
