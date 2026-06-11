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

		// Validate externals for this path
		if p.Depth == DepthEmpty && len(p.Externals) > 0 {
			return fmt.Errorf("paths[%d] %q: cannot declare externals when depth is empty", i, p.Path)
		}
		extSeen := make(map[string]int, len(p.Externals))
		for j, ext := range p.Externals {
			if ext.Target == "" {
				return fmt.Errorf("paths[%d] %q: externals[%d]: target must not be empty", i, p.Path, j)
			}
			if strings.Contains(ext.Target, "/") {
				return fmt.Errorf("paths[%d] %q: externals[%d]: target %q must not contain '/'", i, p.Path, j, ext.Target)
			}
			if ext.Target == ".." {
				return fmt.Errorf("paths[%d] %q: externals[%d]: target must not be '..'", i, p.Path, j)
			}
			if prev, ok := extSeen[ext.Target]; ok {
				return fmt.Errorf("paths[%d] %q: externals[%d]: target %q duplicate of externals[%d]", i, p.Path, j, ext.Target, prev)
			}
			extSeen[ext.Target] = j
		}
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
