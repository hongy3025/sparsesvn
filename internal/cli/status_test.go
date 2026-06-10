package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hongy3025/sparsesvn/internal/config"
	"github.com/hongy3025/sparsesvn/internal/state"
	"github.com/hongy3025/sparsesvn/internal/svn"
)

func TestStatus_InSync(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sparsesvn.yaml")
	yamlContent := "url: svn://server/repo\n" +
		"paths:\n" +
		"  - path: src\n" +
		"    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	hash, err := config.HashFile(cfgPath)
	if err != nil {
		t.Fatalf("hash config: %v", err)
	}

	st := &state.State{
		Version:    state.StateVersion,
		ConfigHash: hash,
		URL:        "svn://server/repo",
		Paths:      []state.PathEntry{{Path: "src", Depth: config.DepthInfinity}},
	}
	if err := state.Save(dir, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	fake := &svn.FakeClient{}
	out := &bytes.Buffer{}

	gf := &GlobalFlags{ConfigFile: cfgPath, Workdir: dir}
	flags := StatusFlags{}

	code := runStatus(t.Context(), gf, flags, fake, out)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "in sync") {
		t.Errorf("stdout %q does not contain 'in sync'", out.String())
	}
}

func TestStatus_HasDiff(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sparsesvn.yaml")
	yamlContent := "url: svn://server/repo\n" +
		"paths:\n" +
		"  - path: src\n" +
		"    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	st := &state.State{
		Version:    state.StateVersion,
		ConfigHash: "sha256:differenthash",
		URL:        "svn://server/repo",
		Paths:      []state.PathEntry{},
	}
	if err := state.Save(dir, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	fake := &svn.FakeClient{}
	out := &bytes.Buffer{}

	gf := &GlobalFlags{ConfigFile: cfgPath, Workdir: dir}
	flags := StatusFlags{}

	code := runStatus(t.Context(), gf, flags, fake, out)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "Plan:") {
		t.Errorf("stdout %q does not contain 'Plan:'", out.String())
	}
}

func TestStatus_JSON_InSync(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sparsesvn.yaml")
	yamlContent := "url: svn://server/repo\n" +
		"paths:\n" +
		"  - path: src\n" +
		"    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	hash, err := config.HashFile(cfgPath)
	if err != nil {
		t.Fatalf("hash config: %v", err)
	}

	st := &state.State{
		Version:    state.StateVersion,
		ConfigHash: hash,
		URL:        "svn://server/repo",
		Paths:      []state.PathEntry{{Path: "src", Depth: config.DepthInfinity}},
	}
	if err := state.Save(dir, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	fake := &svn.FakeClient{}
	out := &bytes.Buffer{}

	gf := &GlobalFlags{ConfigFile: cfgPath, Workdir: dir, JSON: true}
	flags := StatusFlags{}

	code := runStatus(t.Context(), gf, flags, fake, out)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, out.String())
	}
	inSync, ok := raw["in_sync"]
	if !ok {
		t.Errorf("JSON missing 'in_sync' field: %s", out.String())
	}
	if inSync != true {
		t.Errorf("in_sync = %v, want true", inSync)
	}
}

func TestStatus_URLMismatch(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sparsesvn.yaml")
	yamlContent := "url: svn://server/repo\n" +
		"paths:\n" +
		"  - path: src\n" +
		"    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	st := &state.State{
		Version:    state.StateVersion,
		ConfigHash: "sha256:old",
		URL:        "svn://other/repo",
		Paths:      []state.PathEntry{},
	}
	if err := state.Save(dir, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	fake := &svn.FakeClient{}
	out := &bytes.Buffer{}

	gf := &GlobalFlags{ConfigFile: cfgPath, Workdir: dir}
	flags := StatusFlags{}

	code := runStatus(t.Context(), gf, flags, fake, out)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}
