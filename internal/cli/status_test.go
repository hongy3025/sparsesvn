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

	cmd := newRootCmd("test")
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"status", "-f", cfgPath, "-C", dir})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "in sync") {
		t.Errorf("stdout %q does not contain 'in sync'", buf.String())
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

	cmd := newRootCmd("test")
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"status", "-f", cfgPath, "-C", dir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (exit 1) for diff, got nil")
	}
	ee, ok := err.(*exitError)
	if !ok {
		t.Fatalf("error is not *exitError: %v (%T)", err, err)
	}
	if ee.Code != 1 {
		t.Errorf("exit code = %d, want 1", ee.Code)
	}
	if !strings.Contains(buf.String(), "Plan:") {
		t.Errorf("stdout %q does not contain 'Plan:'", buf.String())
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

	cmd := newRootCmd("test")
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"status", "-f", cfgPath, "-C", dir, "--json"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, buf.String())
	}
	inSync, ok := raw["in_sync"]
	if !ok {
		t.Errorf("JSON missing 'in_sync' field: %s", buf.String())
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

	cmd := newRootCmd("test")
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"status", "-f", cfgPath, "-C", dir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for url mismatch, got nil")
	}
	ee, ok := err.(*exitError)
	if !ok {
		t.Fatalf("error is not *exitError: %v (%T)", err, err)
	}
	if ee.Code != 2 {
		t.Errorf("exit code = %d, want 2", ee.Code)
	}
}
