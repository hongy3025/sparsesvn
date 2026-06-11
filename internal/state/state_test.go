package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hongy3025/sparsesvn/internal/config"
)

func TestSaveLoadWithExternals(t *testing.T) {
	dir := t.TempDir()
	s := &State{
		Version:    StateVersion,
		ConfigHash: "sha256:abc123",
		URL:        "svn://server/repo/trunk",
		AppliedAt:  time.Now().UTC().Truncate(time.Second),
		Paths: []PathEntry{
			{
				Path:  "src/core",
				Depth: config.DepthInfinity,
				Externals: []ExternalEntry{
					{Target: "utils", Depth: config.DepthFiles},
					{Target: "proto", Depth: config.DepthInfinity},
				},
			},
			{
				Path:      "docs",
				Depth:     config.DepthFiles,
				Externals: nil,
			},
		},
	}
	if err := Save(dir, s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, exists, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true")
	}
	if loaded.Version != StateVersion {
		t.Errorf("Version = %d, want %d", loaded.Version, StateVersion)
	}
	if len(loaded.Paths) != 2 {
		t.Fatalf("len(Paths) = %d, want 2", len(loaded.Paths))
	}
	// Save sorts paths alphabetically: docs < src/core
	p0 := loaded.Paths[0]
	if p0.Path != "docs" {
		t.Errorf("Paths[0].Path = %q, want %q", p0.Path, "docs")
	}
	if len(p0.Externals) != 0 {
		t.Fatalf("Paths[0].Externals len = %d, want 0", len(p0.Externals))
	}
	p1 := loaded.Paths[1]
	if p1.Path != "src/core" {
		t.Errorf("Paths[1].Path = %q, want %q", p1.Path, "src/core")
	}
	if len(p1.Externals) != 2 {
		t.Fatalf("Paths[1].Externals len = %d, want 2", len(p1.Externals))
	}
	if p1.Externals[0].Target != "utils" {
		t.Errorf("Externals[0].Target = %q", p1.Externals[0].Target)
	}
	if p1.Externals[0].Depth != config.DepthFiles {
		t.Errorf("Externals[0].Depth = %v, want files", p1.Externals[0].Depth)
	}
	if p1.Externals[1].Target != "proto" {
		t.Errorf("Externals[1].Target = %q", p1.Externals[1].Target)
	}
}

func TestLoadVersion1Compat(t *testing.T) {
	v1Yaml := "# sparsesvn state file - DO NOT EDIT MANUALLY\n" +
		"version: 1\n" +
		"config_hash: \"sha256:abc\"\n" +
		"url: \"svn://server/repo/trunk\"\n" +
		"applied_at: 2026-06-11T10:00:00Z\n" +
		"paths:\n" +
		"  - path: src\n" +
		"    depth: infinity\n" +
		"  - path: docs\n" +
		"    depth: files\n"
	dir := t.TempDir()
	statePath := Path(dir)
	os.MkdirAll(filepath.Dir(statePath), 0755)
	os.WriteFile(statePath, []byte(v1Yaml), 0644)

	loaded, exists, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true")
	}
	if loaded.Version != 1 {
		t.Errorf("Version = %d, want 1", loaded.Version)
	}
	for _, p := range loaded.Paths {
		if len(p.Externals) != 0 {
			t.Errorf("Path %q: expected empty externals, got %d", p.Path, len(p.Externals))
		}
	}
}
