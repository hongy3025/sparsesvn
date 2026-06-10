package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlan_TextOutput(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sparsesvn.yaml")
	yaml := "url: svn://server/repo\n" +
		"paths:\n" +
		"  - path: src\n" +
		"    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cmd := newRootCmd("test")
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"plan", "-f", cfgPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Plan:") {
		t.Errorf("stdout %q does not contain 'Plan:'", buf.String())
	}
}

func TestPlan_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sparsesvn.yaml")
	yaml := "url: svn://server/repo\n" +
		"paths:\n" +
		"  - path: src\n" +
		"    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cmd := newRootCmd("test")
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"plan", "-f", cfgPath, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, buf.String())
	}
	if _, ok := raw["url"]; !ok {
		t.Errorf("JSON missing 'url' field: %s", buf.String())
	}
	if _, ok := raw["actions"]; !ok {
		t.Errorf("JSON missing 'actions' field: %s", buf.String())
	}
}

func TestPlan_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(cfgPath, []byte(":::invalid yaml"), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cmd := newRootCmd("test")
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"plan", "-f", cfgPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid config, got nil")
	}
	ee, ok := err.(*exitError)
	if !ok {
		t.Fatalf("error is not *exitError: %v (%T)", err, err)
	}
	if ee.Code != 2 {
		t.Errorf("exit code = %d, want 2", ee.Code)
	}
}
