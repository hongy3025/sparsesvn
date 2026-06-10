package plan

import (
	"slices"
	"testing"

	"github.com/hongy3025/sparsesvn/internal/config"
)

func TestSort_AddsBeforeExcludes(t *testing.T) {
	actions := []Action{
		{Kind: ActionExclude, Path: "src/a"},
		{Kind: ActionAdd, Path: "src"},
		{Kind: ActionExclude, Path: "src/b"},
		{Kind: ActionAdd, Path: "lib"},
	}
	Sort(actions)

	inAddGroup := true
	for _, a := range actions {
		if a.Kind == ActionDowngrade || a.Kind == ActionExclude {
			inAddGroup = false
		}
		if a.Kind == ActionAdd || a.Kind == ActionUpgrade {
			if !inAddGroup {
				t.Errorf("ADD/UPGRADE found after DOWNGRADE/EXCLUDE: %v", a)
			}
		}
	}
}

func TestSort_AddParentBeforeChild(t *testing.T) {
	actions := []Action{
		{Kind: ActionAdd, Path: "src/a/b"},
		{Kind: ActionAdd, Path: "src/a"},
		{Kind: ActionAdd, Path: "src"},
	}
	Sort(actions)

	expected := []string{"src", "src/a", "src/a/b"}
	for i, a := range actions {
		if a.Path != expected[i] {
			t.Errorf("index %d: got %q, want %q", i, a.Path, expected[i])
		}
	}
}

func TestSort_ExcludeChildBeforeParent(t *testing.T) {
	actions := []Action{
		{Kind: ActionExclude, Path: "src"},
		{Kind: ActionExclude, Path: "src/a"},
		{Kind: ActionExclude, Path: "src/a/b"},
	}
	Sort(actions)

	expected := []string{"src/a/b", "src/a", "src"}
	for i, a := range actions {
		if a.Path != expected[i] {
			t.Errorf("index %d: got %q, want %q", i, a.Path, expected[i])
		}
	}
}

func TestSort_LexicographicTieBreaker(t *testing.T) {
	actions := []Action{
		{Kind: ActionAdd, Path: "src/c"},
		{Kind: ActionAdd, Path: "src/a"},
		{Kind: ActionAdd, Path: "src/b"},
	}
	Sort(actions)

	expected := []string{"src/a", "src/b", "src/c"}
	for i, a := range actions {
		if a.Path != expected[i] {
			t.Errorf("index %d: got %q, want %q", i, a.Path, expected[i])
		}
	}
}

func TestSort_Stable(t *testing.T) {
	actions := []Action{
		{Kind: ActionAdd, Path: "src/a", ToDepth: config.DepthFiles},
		{Kind: ActionAdd, Path: "src/a", ToDepth: config.DepthInfinity},
		{Kind: ActionAdd, Path: "src/a", ToDepth: config.DepthEmpty},
	}
	Sort(actions)

	if actions[0].ToDepth != config.DepthFiles {
		t.Error("stable sort: first element changed")
	}
	if actions[1].ToDepth != config.DepthInfinity {
		t.Error("stable sort: second element changed")
	}
	if actions[2].ToDepth != config.DepthEmpty {
		t.Error("stable sort: third element changed")
	}
}

func TestSort_EmptyAndSingle(t *testing.T) {
	var empty []Action
	Sort(empty)
	if len(empty) != 0 {
		t.Error("empty slice should remain empty")
	}

	single := []Action{{Kind: ActionAdd, Path: "src"}}
	Sort(single)
	if len(single) != 1 || single[0].Path != "src" {
		t.Error("single element should be unchanged")
	}
}

func TestSort_DowngradeChildBeforeParent(t *testing.T) {
	actions := []Action{
		{Kind: ActionDowngrade, Path: "src"},
		{Kind: ActionDowngrade, Path: "src/a"},
		{Kind: ActionDowngrade, Path: "src/a/b"},
	}
	Sort(actions)

	expected := []string{"src/a/b", "src/a", "src"}
	for i, a := range actions {
		if a.Path != expected[i] {
			t.Errorf("index %d: got %q, want %q", i, a.Path, expected[i])
		}
	}
}

func TestSort_UpgradeParentBeforeChild(t *testing.T) {
	actions := []Action{
		{Kind: ActionUpgrade, Path: "src/a/b"},
		{Kind: ActionUpgrade, Path: "src/a"},
		{Kind: ActionUpgrade, Path: "src"},
	}
	Sort(actions)

	expected := []string{"src", "src/a", "src/a/b"}
	for i, a := range actions {
		if a.Path != expected[i] {
			t.Errorf("index %d: got %q, want %q", i, a.Path, expected[i])
		}
	}
}

func TestSort_ComplexMixed(t *testing.T) {
	actions := []Action{
		{Kind: ActionExclude, Path: "src/a/b"},
		{Kind: ActionAdd, Path: "src/a/b"},
		{Kind: ActionExclude, Path: "src"},
		{Kind: ActionAdd, Path: "src"},
		{Kind: ActionUpgrade, Path: "lib"},
		{Kind: ActionDowngrade, Path: "lib/sub"},
	}
	Sort(actions)

	addUpgrade := []ActionKind{ActionAdd, ActionUpgrade}
	excludeDowngrade := []ActionKind{ActionExclude, ActionDowngrade}

	inAddGroup := true
	for _, a := range actions {
		if slices.Contains(excludeDowngrade, a.Kind) {
			inAddGroup = false
		}
		if slices.Contains(addUpgrade, a.Kind) && !inAddGroup {
			t.Errorf("ADD/UPGRADE found after DOWNGRADE/EXCLUDE: %v", a)
		}
	}
}
