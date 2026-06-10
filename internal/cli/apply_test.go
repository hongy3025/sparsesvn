package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sparsesvn/sparsesvn/internal/config"
	"github.com/sparsesvn/sparsesvn/internal/state"
	"github.com/sparsesvn/sparsesvn/internal/svn"
)

func TestApply_InvalidConfig(t *testing.T) {
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
	cmd.SetArgs([]string{"apply", "-f", cfgPath, "-C", dir})

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

func TestApply_URLRequired(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sparsesvn.yaml")
	yamlContent := "paths:\n" +
		"  - path: src\n" +
		"    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cmd := newRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"apply", "-f", cfgPath, "-C", dir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing URL, got nil")
	}
	ee, ok := err.(*exitError)
	if !ok {
		t.Fatalf("error is not *exitError: %v (%T)", err, err)
	}
	if ee.Code != 2 {
		t.Errorf("exit code = %d, want 2", ee.Code)
	}
}

func TestApply_DryRun_OutputsPlan(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sparsesvn.yaml")
	yamlContent := "url: svn://server/repo\n" +
		"paths:\n" +
		"  - path: src\n" +
		"    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	fake := &svn.FakeClient{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	gf := &GlobalFlags{ConfigFile: cfgPath, Workdir: dir}
	flags := ApplyFlags{DryRun: true}

	code := runApply(t.Context(), gf, flags, fake, out, errOut)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errOut.String())
	}
	if len(fake.Calls) != 0 {
		t.Errorf("expected 0 svn calls for dry-run, got %d", len(fake.Calls))
	}
	if !strings.Contains(out.String(), "Plan:") {
		t.Errorf("stdout %q does not contain 'Plan:'", out.String())
	}
}

func TestApply_FreshCheckout_Success(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sparsesvn.yaml")
	yamlContent := "url: svn://server/repo\n" +
		"paths:\n" +
		"  - path: src\n" +
		"    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	fake := &svn.FakeClient{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	gf := &GlobalFlags{ConfigFile: cfgPath, Workdir: dir}
	flags := ApplyFlags{}

	code := runApply(t.Context(), gf, flags, fake, out, errOut)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Applied") {
		t.Errorf("stdout %q does not contain 'Applied'", out.String())
	}

	st, exists, err := state.Load(dir)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if !exists {
		t.Fatal("state file not written")
	}
	if st.URL != "svn://server/repo" {
		t.Errorf("state URL = %q, want %q", st.URL, "svn://server/repo")
	}
	if len(st.Paths) != 1 || st.Paths[0].Path != "src" {
		t.Errorf("state paths = %v, want [{src infinity}]", st.Paths)
	}
}

func TestApply_SvnFailure(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sparsesvn.yaml")
	yamlContent := "url: svn://server/repo\n" +
		"paths:\n" +
		"  - path: src\n" +
		"    depth: infinity\n" +
		"  - path: lib\n" +
		"    depth: infinity\n"
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	fake := &svn.FakeClient{
		FailOn: []svn.FakeFailRule{
			{ArgsContains: []string{"set-depth", "lib"}, Stderr: "permission denied", ExitCode: 1},
		},
	}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	gf := &GlobalFlags{ConfigFile: cfgPath, Workdir: dir}
	flags := ApplyFlags{}

	code := runApply(t.Context(), gf, flags, fake, out, errOut)
	if code != 3 {
		t.Errorf("exit code = %d, want 3; stderr: %s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "permission denied") {
		t.Errorf("stderr %q does not contain 'permission denied'", errOut.String())
	}

	st, exists, err := state.Load(dir)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if !exists {
		t.Fatal("half-state file not written")
	}
	if st.ConfigHash != "" {
		t.Errorf("half-state ConfigHash = %q, want empty", st.ConfigHash)
	}
}

func TestApply_URLMismatch(t *testing.T) {
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
	errOut := &bytes.Buffer{}

	gf := &GlobalFlags{ConfigFile: cfgPath, Workdir: dir}
	flags := ApplyFlags{}

	code := runApply(t.Context(), gf, flags, fake, out, errOut)
	if code != 2 {
		t.Errorf("exit code = %d, want 2; stderr: %s", code, errOut.String())
	}
}

func TestApply_FastPath(t *testing.T) {
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
	errOut := &bytes.Buffer{}

	gf := &GlobalFlags{ConfigFile: cfgPath, Workdir: dir}
	flags := ApplyFlags{}

	code := runApply(t.Context(), gf, flags, fake, out, errOut)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Already in sync") {
		t.Errorf("stdout %q does not contain 'Already in sync'", out.String())
	}
	if len(fake.Calls) != 0 {
		t.Errorf("expected 0 svn calls for fast path, got %d", len(fake.Calls))
	}
}
