package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Depth int

const (
	DepthEmpty Depth = iota
	DepthFiles
	DepthInfinity
)

func (d Depth) String() string {
	switch d {
	case DepthEmpty:
		return "empty"
	case DepthFiles:
		return "files"
	case DepthInfinity:
		return "infinity"
	default:
		return fmt.Sprintf("unknown(%d)", int(d))
	}
}

func ParseDepth(s string) (Depth, error) {
	switch s {
	case "empty":
		return DepthEmpty, nil
	case "files":
		return DepthFiles, nil
	case "infinity":
		return DepthInfinity, nil
	default:
		return 0, fmt.Errorf("invalid depth %q (want empty|files|infinity)", s)
	}
}

type PathSpec struct {
	Path  string
	Depth Depth
}

type Config struct {
	URL   string
	Paths []PathSpec
}

type rawPathSpec struct {
	Path  string `yaml:"path"`
	Depth string `yaml:"depth"`
}

type rawConfig struct {
	URL   string        `yaml:"url"`
	Paths []rawPathSpec `yaml:"paths"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load config %s: %w", path, err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("load config %s: parse yaml: %w", path, err)
	}

	cfg := &Config{
		URL:   raw.URL,
		Paths: make([]PathSpec, 0, len(raw.Paths)),
	}
	for i, rp := range raw.Paths {
		d, err := ParseDepth(rp.Depth)
		if err != nil {
			return nil, fmt.Errorf("load config %s: paths[%d]: %w", path, i, err)
		}
		cfg.Paths = append(cfg.Paths, PathSpec{Path: rp.Path, Depth: d})
	}
	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("load config %s: %w", path, err)
	}
	return cfg, nil
}
