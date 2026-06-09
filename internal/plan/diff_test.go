package plan

import (
	"reflect"
	"testing"

	"github.com/sparsesvn/sparsesvn/internal/config"
)

func toMap(actions []Action) map[string]Action {
	m := make(map[string]Action, len(actions))
	for _, a := range actions {
		m[a.Path] = a
	}
	return m
}

func TestDiff_AllNew(t *testing.T) {
	desired := map[string]config.Depth{
		"a": config.DepthInfinity,
		"b": config.DepthFiles,
		"c": config.DepthEmpty,
	}
	got := Diff(desired, nil)
	if len(got) != 3 {
		t.Fatalf("want 3 actions, got %d: %+v", len(got), got)
	}
	m := toMap(got)
	want := map[string]Action{
		"a": {Kind: ActionAdd, Path: "a", ToDepth: config.DepthInfinity},
		"b": {Kind: ActionAdd, Path: "b", ToDepth: config.DepthFiles},
		"c": {Kind: ActionAdd, Path: "c", ToDepth: config.DepthEmpty},
	}
	if !reflect.DeepEqual(m, want) {
		t.Fatalf("mismatch\n got: %+v\nwant: %+v", m, want)
	}
}

func TestDiff_AllRemoved(t *testing.T) {
	current := map[string]config.Depth{
		"a": config.DepthInfinity,
		"b": config.DepthFiles,
		"c": config.DepthEmpty,
	}
	got := Diff(nil, current)
	if len(got) != 3 {
		t.Fatalf("want 3 actions, got %d: %+v", len(got), got)
	}
	m := toMap(got)
	want := map[string]Action{
		"a": {Kind: ActionExclude, Path: "a", FromDepth: config.DepthInfinity},
		"b": {Kind: ActionExclude, Path: "b", FromDepth: config.DepthFiles},
		"c": {Kind: ActionExclude, Path: "c", FromDepth: config.DepthEmpty},
	}
	if !reflect.DeepEqual(m, want) {
		t.Fatalf("mismatch\n got: %+v\nwant: %+v", m, want)
	}
}

func TestDiff_Identical(t *testing.T) {
	m := map[string]config.Depth{
		"a": config.DepthInfinity,
		"b": config.DepthFiles,
		"c": config.DepthEmpty,
	}
	desired := map[string]config.Depth{
		"a": config.DepthInfinity,
		"b": config.DepthFiles,
		"c": config.DepthEmpty,
	}
	got := Diff(desired, m)
	if len(got) != 0 {
		t.Fatalf("want empty slice, got %d: %+v", len(got), got)
	}
}

func TestDiff_DepthChanges(t *testing.T) {
	cases := []struct {
		name string
		from config.Depth
		to   config.Depth
		want *Action
	}{
		{"empty->files", config.DepthEmpty, config.DepthFiles,
			&Action{Kind: ActionUpgrade, Path: "p", FromDepth: config.DepthEmpty, ToDepth: config.DepthFiles}},
		{"empty->infinity", config.DepthEmpty, config.DepthInfinity,
			&Action{Kind: ActionUpgrade, Path: "p", FromDepth: config.DepthEmpty, ToDepth: config.DepthInfinity}},
		{"files->infinity", config.DepthFiles, config.DepthInfinity,
			&Action{Kind: ActionUpgrade, Path: "p", FromDepth: config.DepthFiles, ToDepth: config.DepthInfinity}},
		{"files->empty", config.DepthFiles, config.DepthEmpty,
			&Action{Kind: ActionDowngrade, Path: "p", FromDepth: config.DepthFiles, ToDepth: config.DepthEmpty}},
		{"infinity->empty", config.DepthInfinity, config.DepthEmpty,
			&Action{Kind: ActionDowngrade, Path: "p", FromDepth: config.DepthInfinity, ToDepth: config.DepthEmpty}},
		{"infinity->files", config.DepthInfinity, config.DepthFiles,
			&Action{Kind: ActionDowngrade, Path: "p", FromDepth: config.DepthInfinity, ToDepth: config.DepthFiles}},
		{"empty->empty", config.DepthEmpty, config.DepthEmpty, nil},
		{"files->files", config.DepthFiles, config.DepthFiles, nil},
		{"infinity->infinity", config.DepthInfinity, config.DepthInfinity, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			desired := map[string]config.Depth{"p": tc.to}
			current := map[string]config.Depth{"p": tc.from}
			got := Diff(desired, current)
			if tc.want == nil {
				if len(got) != 0 {
					t.Fatalf("want NOOP (empty), got %+v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("want 1 action, got %d: %+v", len(got), got)
			}
			if !reflect.DeepEqual(got[0], *tc.want) {
				t.Fatalf("mismatch\n got: %+v\nwant: %+v", got[0], *tc.want)
			}
		})
	}
}

func TestDiff_MixedScenario(t *testing.T) {
	desired := map[string]config.Depth{
		"a": config.DepthInfinity,
		"b": config.DepthFiles,
		"c": config.DepthEmpty,
		"d": config.DepthEmpty,
	}
	current := map[string]config.Depth{
		"b": config.DepthEmpty,
		"c": config.DepthInfinity,
		"d": config.DepthEmpty,
		"e": config.DepthFiles,
	}
	got := Diff(desired, current)
	m := toMap(got)
	want := map[string]Action{
		"a": {Kind: ActionAdd, Path: "a", ToDepth: config.DepthInfinity},
		"b": {Kind: ActionUpgrade, Path: "b", FromDepth: config.DepthEmpty, ToDepth: config.DepthFiles},
		"c": {Kind: ActionDowngrade, Path: "c", FromDepth: config.DepthInfinity, ToDepth: config.DepthEmpty},
		"e": {Kind: ActionExclude, Path: "e", FromDepth: config.DepthFiles},
	}
	if !reflect.DeepEqual(m, want) {
		t.Fatalf("mismatch\n got: %+v\nwant: %+v", m, want)
	}
}
