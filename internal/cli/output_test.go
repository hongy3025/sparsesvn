package cli

import (
	"encoding/json"
	"testing"

	"github.com/hongy3025/sparsesvn/internal/config"
	"github.com/hongy3025/sparsesvn/internal/plan"
)

func TestFormatPlan_Empty(t *testing.T) {
	got := FormatPlan(nil)
	want := "Plan: 0 actions (no changes)"
	if got != want {
		t.Errorf("FormatPlan(nil) = %q, want %q", got, want)
	}

	got = FormatPlan([]plan.Action{})
	if got != want {
		t.Errorf("FormatPlan([]) = %q, want %q", got, want)
	}
}

func TestFormatPlan_AllKinds(t *testing.T) {
	actions := []plan.Action{
		{Kind: plan.ActionAdd, Path: "src", FromDepth: config.DepthEmpty, ToDepth: config.DepthInfinity},
		{Kind: plan.ActionAdd, Path: "src/core", FromDepth: config.DepthEmpty, ToDepth: config.DepthInfinity},
		{Kind: plan.ActionUpgrade, Path: "docs", FromDepth: config.DepthEmpty, ToDepth: config.DepthFiles},
		{Kind: plan.ActionDowngrade, Path: "old_module", FromDepth: config.DepthInfinity, ToDepth: config.DepthEmpty},
		{Kind: plan.ActionExclude, Path: "tmp", FromDepth: config.DepthFiles, ToDepth: config.DepthEmpty},
	}

	got := FormatPlan(actions)

	if !contains(got, "Plan: 5 actions") {
		t.Errorf("summary line missing or wrong in:\n%s", got)
	}
	if !contains(got, "+") {
		t.Errorf("missing '+' marker in:\n%s", got)
	}
	if !contains(got, "~") {
		t.Errorf("missing '~' marker in:\n%s", got)
	}
	if !contains(got, "-") {
		t.Errorf("missing '-' marker in:\n%s", got)
	}
	if !contains(got, "ADD") {
		t.Errorf("missing 'ADD' in:\n%s", got)
	}
	if !contains(got, "UPGRADE") {
		t.Errorf("missing 'UPGRADE' in:\n%s", got)
	}
	if !contains(got, "DOWNGRADE") {
		t.Errorf("missing 'DOWNGRADE' in:\n%s", got)
	}
	if !contains(got, "EXCLUDE") {
		t.Errorf("missing 'EXCLUDE' in:\n%s", got)
	}
}

func TestBuildPlanJSON_Summary(t *testing.T) {
	actions := []plan.Action{
		{Kind: plan.ActionAdd, Path: "src", ToDepth: config.DepthInfinity},
		{Kind: plan.ActionAdd, Path: "docs", ToDepth: config.DepthFiles},
		{Kind: plan.ActionUpgrade, Path: "lib", FromDepth: config.DepthEmpty, ToDepth: config.DepthFiles},
		{Kind: plan.ActionDowngrade, Path: "old", FromDepth: config.DepthInfinity, ToDepth: config.DepthEmpty},
		{Kind: plan.ActionExclude, Path: "tmp", FromDepth: config.DepthFiles},
	}

	pj := BuildPlanJSON("svn://server/repo", actions)

	if pj.Summary.Add != 2 {
		t.Errorf("Summary.Add = %d, want 2", pj.Summary.Add)
	}
	if pj.Summary.Upgrade != 1 {
		t.Errorf("Summary.Upgrade = %d, want 1", pj.Summary.Upgrade)
	}
	if pj.Summary.Downgrade != 1 {
		t.Errorf("Summary.Downgrade = %d, want 1", pj.Summary.Downgrade)
	}
	if pj.Summary.Exclude != 1 {
		t.Errorf("Summary.Exclude = %d, want 1", pj.Summary.Exclude)
	}
	if pj.Summary.Total != 5 {
		t.Errorf("Summary.Total = %d, want 5", pj.Summary.Total)
	}
	if pj.Url != "svn://server/repo" {
		t.Errorf("Url = %q, want svn://server/repo", pj.Url)
	}
}

func TestPlanJSON_Marshal(t *testing.T) {
	actions := []plan.Action{
		{Kind: plan.ActionAdd, Path: "src", ToDepth: config.DepthInfinity},
		{Kind: plan.ActionExclude, Path: "tmp", FromDepth: config.DepthFiles},
	}

	pj := BuildPlanJSON("svn://server/repo", actions)
	data, err := json.Marshal(pj)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	s := string(data)

	if !contains(s, `"url":"svn://server/repo"`) {
		t.Errorf("json missing url field: %s", s)
	}
	if !contains(s, `"kind":"ADD"`) {
		t.Errorf("json missing ADD kind: %s", s)
	}
	if !contains(s, `"kind":"EXCLUDE"`) {
		t.Errorf("json missing EXCLUDE kind: %s", s)
	}
	if !contains(s, `"path":"src"`) {
		t.Errorf("json missing path src: %s", s)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	actionsArr := raw["actions"].([]interface{})
	addAction := actionsArr[0].(map[string]interface{})
	if _, ok := addAction["from_depth"]; ok {
		t.Errorf("ADD action should omit from_depth, but found it in: %s", s)
	}

	excludeAction := actionsArr[1].(map[string]interface{})
	if _, ok := excludeAction["to_depth"]; ok {
		t.Errorf("EXCLUDE action should omit to_depth, but found it in: %s", s)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
