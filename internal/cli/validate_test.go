package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate_OKConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sparsesvn.yaml")
	yaml := "url: svn://server/repo\n" +
		"paths:\n" +
		"  - path: src\n" +
		"    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cmd := newRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"validate", "-f", cfgPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("stdout %q does not contain OK", buf.String())
	}
	if !strings.Contains(buf.String(), cfgPath) {
		t.Errorf("stdout %q does not contain path %q", buf.String(), cfgPath)
	}
}

func TestValidate_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(cfgPath, []byte(":::invalid yaml"), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cmd := newRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"validate", "-f", cfgPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid yaml, got nil")
	}
	ee, ok := err.(*exitError)
	if !ok {
		t.Fatalf("error is not *exitError: %v (%T)", err, err)
	}
	if ee.Code != 2 {
		t.Errorf("exit code = %d, want 2", ee.Code)
	}
	if !strings.Contains(ee.Error(), "parse yaml") {
		t.Errorf("error %q does not contain 'parse yaml'", ee.Error())
	}
}

func TestValidate_PathRuleViolation(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	yaml := "url: svn://server/repo\n" +
		"paths:\n" +
		"  - path: /src\n" +
		"    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cmd := newRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"validate", "-f", cfgPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for path rule violation, got nil")
	}
	ee, ok := err.(*exitError)
	if !ok {
		t.Fatalf("error is not *exitError: %v (%T)", err, err)
	}
	if ee.Code != 2 {
		t.Errorf("exit code = %d, want 2", ee.Code)
	}
}
