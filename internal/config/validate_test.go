package config

import (
	"strings"
	"testing"
)

func TestValidate_OK(t *testing.T) {
	cfg := &Config{
		URL: "svn://server/repo/trunk",
		Paths: []PathSpec{
			{Path: "src", Depth: DepthInfinity},
			{Path: "docs/api", Depth: DepthFiles},
			{Path: "vendor/foo.bar", Depth: DepthEmpty},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestValidate_NilConfig(t *testing.T) {
	if err := Validate(nil); err == nil {
		t.Fatal("Validate(nil) err = nil, want non-nil")
	}
}

func TestValidate_EmptyPaths(t *testing.T) {
	cfg := &Config{URL: "svn://x/y", Paths: nil}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate err = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "paths") {
		t.Errorf("err %q does not mention %q", err.Error(), "paths")
	}
}

func TestValidate_InvalidPath(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"empty", ""},
		{"leading-slash", "/src"},
		{"trailing-slash", "src/"},
		{"dot-dot", "src/../etc"},
		{"only-dot-dot", ".."},
		{"backslash", `src\foo`},
		{"double-slash", "src//foo"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := &Config{
				Paths: []PathSpec{{Path: c.path, Depth: DepthInfinity}},
			}
			err := Validate(cfg)
			if err == nil {
				t.Fatalf("Validate(path=%q) err = nil, want non-nil", c.path)
			}
			if !strings.Contains(err.Error(), "paths[0]") {
				t.Errorf("err %q does not contain %q", err.Error(), "paths[0]")
			}
		})
	}
}

func TestValidate_DuplicatePath(t *testing.T) {
	cfg := &Config{
		Paths: []PathSpec{
			{Path: "src", Depth: DepthInfinity},
			{Path: "src", Depth: DepthFiles},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate err = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("err %q does not mention %q", err.Error(), "duplicate")
	}
}

func TestValidate_URLOptional(t *testing.T) {
	cfg := &Config{
		URL: "",
		Paths: []PathSpec{
			{Path: "src", Depth: DepthInfinity},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate with empty URL returned error: %v", err)
	}
}
