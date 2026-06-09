package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_ValidMinimal(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sparsesvn.yaml")
	yaml := "" +
		"url: svn://server/repo/trunk\n" +
		"paths:\n" +
		"  - path: src\n" +
		"    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write tmp yaml: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load returned nil config")
	}
	if cfg.URL != "svn://server/repo/trunk" {
		t.Errorf("URL = %q, want svn://server/repo/trunk", cfg.URL)
	}
	if len(cfg.Paths) != 1 {
		t.Fatalf("len(Paths) = %d, want 1", len(cfg.Paths))
	}
	if cfg.Paths[0].Path != "src" {
		t.Errorf("Paths[0].Path = %q, want src", cfg.Paths[0].Path)
	}
	if cfg.Paths[0].Depth != DepthInfinity {
		t.Errorf("Paths[0].Depth = %v, want DepthInfinity", cfg.Paths[0].Depth)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.yaml")

	cfg, err := Load(missing)
	if err == nil {
		t.Fatal("Load(missing) returned nil error, want non-nil")
	}
	if cfg != nil {
		t.Errorf("Load(missing) returned non-nil config: %+v", cfg)
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error %q does not contain path %q", err.Error(), missing)
	}
}

func TestDepthString(t *testing.T) {
	cases := []struct {
		d    Depth
		want string
	}{
		{DepthEmpty, "empty"},
		{DepthFiles, "files"},
		{DepthInfinity, "infinity"},
	}
	for _, c := range cases {
		if got := c.d.String(); got != c.want {
			t.Errorf("Depth(%d).String() = %q, want %q", int(c.d), got, c.want)
		}
	}
}

func TestParseDepth(t *testing.T) {
	okCases := []struct {
		in   string
		want Depth
	}{
		{"empty", DepthEmpty},
		{"files", DepthFiles},
		{"infinity", DepthInfinity},
	}
	for _, c := range okCases {
		got, err := ParseDepth(c.in)
		if err != nil {
			t.Errorf("ParseDepth(%q) err = %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseDepth(%q) = %v, want %v", c.in, got, c.want)
		}
	}

	badCases := []string{"", "Empty", "immediates", "infinite", "INFINITY"}
	for _, in := range badCases {
		if _, err := ParseDepth(in); err == nil {
			t.Errorf("ParseDepth(%q) err = nil, want non-nil", in)
		}
	}
}
