package plan

import (
	"reflect"
	"testing"

	"github.com/hongy3025/sparsesvn/internal/config"
)

func TestExpand_SinglePathInfinity(t *testing.T) {
	cfg := &config.Config{
		Paths: []config.PathSpec{
			{Path: "src/core/utils", Depth: config.DepthInfinity},
		},
	}
	got := Expand(cfg)
	want := map[string]config.Depth{
		"src":            config.DepthEmpty,
		"src/core":       config.DepthEmpty,
		"src/core/utils": config.DepthInfinity,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand mismatch\ngot:  %v\nwant: %v", got, want)
	}
}

func TestExpand_NoParents(t *testing.T) {
	cfg := &config.Config{
		Paths: []config.PathSpec{
			{Path: "src", Depth: config.DepthInfinity},
		},
	}
	got := Expand(cfg)
	want := map[string]config.Depth{"src": config.DepthInfinity}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand mismatch\ngot:  %v\nwant: %v", got, want)
	}
}

func TestExpand_ExplicitParentDeeper(t *testing.T) {
	cfg := &config.Config{
		Paths: []config.PathSpec{
			{Path: "src", Depth: config.DepthFiles},
			{Path: "src/core", Depth: config.DepthInfinity},
		},
	}
	got := Expand(cfg)
	want := map[string]config.Depth{
		"src":      config.DepthFiles,
		"src/core": config.DepthInfinity,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand mismatch\ngot:  %v\nwant: %v", got, want)
	}
}

func TestExpand_SiblingsShareParent(t *testing.T) {
	cfg := &config.Config{
		Paths: []config.PathSpec{
			{Path: "src/a", Depth: config.DepthFiles},
			{Path: "src/b", Depth: config.DepthInfinity},
		},
	}
	got := Expand(cfg)
	want := map[string]config.Depth{
		"src":   config.DepthEmpty,
		"src/a": config.DepthFiles,
		"src/b": config.DepthInfinity,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand mismatch\ngot:  %v\nwant: %v", got, want)
	}
}

func TestExpand_EmptyConfig(t *testing.T) {
	cfg := &config.Config{}
	got := Expand(cfg)
	if len(got) != 0 {
		t.Fatalf("Expand on empty cfg = %v, want empty map", got)
	}
}

func TestExpand_ExplicitParentSameDepth(t *testing.T) {
	cfg := &config.Config{
		Paths: []config.PathSpec{
			{Path: "src", Depth: config.DepthEmpty},
			{Path: "src/a", Depth: config.DepthFiles},
		},
	}
	got := Expand(cfg)
	want := map[string]config.Depth{
		"src":   config.DepthEmpty,
		"src/a": config.DepthFiles,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand mismatch\ngot:  %v\nwant: %v", got, want)
	}
}

func TestExpand_DeepNesting(t *testing.T) {
	cfg := &config.Config{
		Paths: []config.PathSpec{
			{Path: "a/b/c/d/e", Depth: config.DepthFiles},
		},
	}
	got := Expand(cfg)
	want := map[string]config.Depth{
		"a":         config.DepthEmpty,
		"a/b":       config.DepthEmpty,
		"a/b/c":     config.DepthEmpty,
		"a/b/c/d":   config.DepthEmpty,
		"a/b/c/d/e": config.DepthFiles,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand mismatch\ngot:  %v\nwant: %v", got, want)
	}
}
