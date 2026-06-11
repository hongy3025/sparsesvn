package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithExternals(t *testing.T) {
	yaml := `url: svn://server/repo/trunk
paths:
  - path: src/core
    depth: infinity
    externals:
      - target: utils
        depth: files
      - target: proto
        depth: infinity
  - path: docs
    depth: files
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sparsesvn.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(cfg.Paths))
	}
	p0 := cfg.Paths[0]
	if p0.Path != "src/core" {
		t.Errorf("path[0].Path = %q, want %q", p0.Path, "src/core")
	}
	if p0.Depth != DepthInfinity {
		t.Errorf("path[0].Depth = %v, want infinity", p0.Depth)
	}
	if len(p0.Externals) != 2 {
		t.Fatalf("path[0].Externals len = %d, want 2", len(p0.Externals))
	}
	if p0.Externals[0].Target != "utils" {
		t.Errorf("externals[0].Target = %q, want %q", p0.Externals[0].Target, "utils")
	}
	if p0.Externals[0].Depth != DepthFiles {
		t.Errorf("externals[0].Depth = %v, want files", p0.Externals[0].Depth)
	}
	if p0.Externals[1].Target != "proto" {
		t.Errorf("externals[1].Target = %q, want %q", p0.Externals[1].Target, "proto")
	}
	if p0.Externals[1].Depth != DepthInfinity {
		t.Errorf("externals[1].Depth = %v, want infinity", p0.Externals[1].Depth)
	}
	p1 := cfg.Paths[1]
	if len(p1.Externals) != 0 {
		t.Errorf("path[1].Externals len = %d, want 0", len(p1.Externals))
	}
}

func TestLoadWithoutExternals(t *testing.T) {
	yaml := `url: svn://server/repo/trunk
paths:
  - path: src/core
    depth: infinity
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sparsesvn.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Paths[0].Externals) != 0 {
		t.Errorf("expected empty externals, got %d", len(cfg.Paths[0].Externals))
	}
}
