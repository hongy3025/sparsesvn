// internal/plan/expand_test.go
package plan

import (
	"testing"

	"github.com/hongy3025/sparsesvn/internal/config"
)

func TestExpandWithExternals(t *testing.T) {
	cfg := &config.Config{
		URL: "svn://server/repo/trunk",
		Paths: []config.PathSpec{
			{
				Path:  "src/core",
				Depth: config.DepthInfinity,
				Externals: []config.ExternalSpec{
					{Target: "utils", Depth: config.DepthFiles},
					{Target: "proto", Depth: config.DepthInfinity},
				},
			},
			{Path: "docs", Depth: config.DepthFiles},
		},
	}
	result := Expand(cfg)

	// Check path expansion
	if result.Paths["src"] != config.DepthEmpty {
		t.Errorf("src depth = %v, want empty", result.Paths["src"])
	}
	if result.Paths["src/core"] != config.DepthInfinity {
		t.Errorf("src/core depth = %v, want infinity", result.Paths["src/core"])
	}
	if result.Paths["docs"] != config.DepthFiles {
		t.Errorf("docs depth = %v, want files", result.Paths["docs"])
	}

	// Check externals
	if len(result.Externals["src/core"]) != 2 {
		t.Fatalf("src/core externals len = %d, want 2", len(result.Externals["src/core"]))
	}
	if result.Externals["src/core"][0].Target != "utils" {
		t.Errorf("externals[0].Target = %q", result.Externals["src/core"][0].Target)
	}
	if result.Externals["src/core"][0].Depth != config.DepthFiles {
		t.Errorf("externals[0].Depth = %v, want files", result.Externals["src/core"][0].Depth)
	}
	if len(result.Externals["docs"]) != 0 {
		t.Errorf("docs externals len = %d, want 0", len(result.Externals["docs"]))
	}
}

func TestExpandWithoutExternals(t *testing.T) {
	cfg := &config.Config{
		URL:   "svn://server/repo/trunk",
		Paths: []config.PathSpec{{Path: "src", Depth: config.DepthInfinity}},
	}
	result := Expand(cfg)
	if len(result.Externals["src"]) != 0 {
		t.Errorf("expected empty externals for src, got %d", len(result.Externals["src"]))
	}
}