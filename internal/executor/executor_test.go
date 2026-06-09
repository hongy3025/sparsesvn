package executor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sparsesvn/sparsesvn/internal/logx"
	"github.com/sparsesvn/sparsesvn/internal/plan"
	"github.com/sparsesvn/sparsesvn/internal/state"
	"github.com/sparsesvn/sparsesvn/internal/svn"
)

func writeConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "sparse.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func newLogger() *logx.Logger {
	return logx.New(os.Stderr, logx.LevelQuiet, false)
}

func TestApply_FreshCheckout(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, dir, `url: svn://example.com/repo
paths:
  - path: trunk/src
    depth: infinity
`)
	fake := &svn.FakeClient{}
	opts := Options{
		ConfigPath: cfgPath,
		Workdir:    workdir,
		Client:     fake,
		Logger:     newLogger(),
	}

	result := Apply(context.Background(), opts)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.FastPath {
		t.Error("expected FastPath=false")
	}
	if result.ExecutedCount == 0 {
		t.Error("expected ExecutedCount > 0")
	}
	// Should have checkout + at least one set-depth
	if len(fake.Calls) < 2 {
		t.Fatalf("expected at least 2 calls, got %d: %v", len(fake.Calls), fake.Calls)
	}
	// First call should be checkout
	if fake.Calls[0].Args[0] != "checkout" {
		t.Errorf("first call should be checkout, got %v", fake.Calls[0].Args)
	}
	// Remaining calls should be update --set-depth
	for i := 1; i < len(fake.Calls); i++ {
		if fake.Calls[i].Args[0] != "update" {
			t.Errorf("call %d should be update, got %v", i, fake.Calls[i].Args)
		}
	}
}

func TestApply_FastPath(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, dir, `url: svn://example.com/repo
paths:
  - path: trunk/src
    depth: empty
`)
	fake := &svn.FakeClient{}
	ctx := context.Background()
	opts := Options{
		ConfigPath: cfgPath,
		Workdir:    workdir,
		Client:     fake,
		Logger:     newLogger(),
	}

	// First apply to write state
	r1 := Apply(ctx, opts)
	if r1.Err != nil {
		t.Fatalf("first apply error: %v", r1.Err)
	}
	callCount := len(fake.Calls)
	if callCount == 0 {
		t.Fatal("expected some svn calls on first apply")
	}

	// Second apply - should hit fast path
	fake.Reset()
	r2 := Apply(ctx, opts)
	if r2.Err != nil {
		t.Fatalf("second apply error: %v", r2.Err)
	}
	if !r2.FastPath {
		t.Error("expected FastPath=true on second apply")
	}
	if r2.ExecutedCount != 0 {
		t.Errorf("expected ExecutedCount=0, got %d", r2.ExecutedCount)
	}
	if len(fake.Calls) != 0 {
		t.Errorf("expected no new svn calls, got %d", len(fake.Calls))
	}
}

func TestApply_URLMismatch(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, dir, `url: svn://example.com/repo
paths:
  - path: trunk/src
    depth: empty
`)
	fake := &svn.FakeClient{}
	ctx := context.Background()

	// First apply with one URL
	opts1 := Options{
		ConfigPath: cfgPath,
		Workdir:    workdir,
		Client:     fake,
		Logger:     newLogger(),
	}
	r1 := Apply(ctx, opts1)
	if r1.Err != nil {
		t.Fatalf("first apply error: %v", r1.Err)
	}

	// Second apply with different URL override
	opts2 := Options{
		ConfigPath:  cfgPath,
		Workdir:     workdir,
		URLOverride: "svn://other.com/repo",
		Client:      fake,
		Logger:      newLogger(),
	}
	r2 := Apply(ctx, opts2)
	if r2.Err == nil {
		t.Fatal("expected error for URL mismatch")
	}
	if !strings.Contains(r2.Err.Error(), "url mismatch") {
		t.Errorf("error should contain 'url mismatch', got: %v", r2.Err)
	}
	if r2.ExecutedCount != 0 {
		t.Errorf("expected ExecutedCount=0, got %d", r2.ExecutedCount)
	}
}

func TestApply_URLRequired(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, dir, `paths:
  - path: trunk/src
    depth: empty
`)
	fake := &svn.FakeClient{}
	opts := Options{
		ConfigPath: cfgPath,
		Workdir:    workdir,
		Client:     fake,
		Logger:     newLogger(),
	}

	result := Apply(context.Background(), opts)
	if result.Err == nil {
		t.Fatal("expected error for missing URL")
	}
	if !strings.Contains(result.Err.Error(), "url required") {
		t.Errorf("error should contain 'url required', got: %v", result.Err)
	}
}

func TestApply_AddNewPath(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	// Config with src only
	cfgPath := writeConfig(t, dir, `url: svn://example.com/repo
paths:
  - path: trunk/src
    depth: empty
`)
	fake := &svn.FakeClient{}
	ctx := context.Background()
	opts := Options{
		ConfigPath: cfgPath,
		Workdir:    workdir,
		Client:     fake,
		Logger:     newLogger(),
	}

	// First apply
	r1 := Apply(ctx, opts)
	if r1.Err != nil {
		t.Fatalf("first apply error: %v", r1.Err)
	}

	// Add docs to config
	cfgPath = writeConfig(t, dir, `url: svn://example.com/repo
paths:
  - path: trunk/src
    depth: empty
  - path: trunk/docs
    depth: empty
`)
	// Need to re-hash, so use new config path
	fake.Reset()
	opts.ConfigPath = cfgPath
	r2 := Apply(ctx, opts)
	if r2.Err != nil {
		t.Fatalf("second apply error: %v", r2.Err)
	}
	if r2.FastPath {
		t.Error("expected FastPath=false")
	}
	// Should execute exactly 1 set-depth for the new path
	setDepthCalls := 0
	for _, call := range fake.Calls {
		if len(call.Args) >= 3 && call.Args[0] == "update" && call.Args[1] == "--set-depth" {
			setDepthCalls++
		}
	}
	if setDepthCalls != 1 {
		t.Errorf("expected 1 set-depth call, got %d; calls: %v", setDepthCalls, fake.Calls)
	}
	if r2.ExecutedCount != 1 {
		t.Errorf("expected ExecutedCount=1, got %d", r2.ExecutedCount)
	}
}

func TestApply_DowngradeAndExclude(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	// First apply: src=infinity, docs=files
	cfgPath := writeConfig(t, dir, `url: svn://example.com/repo
paths:
  - path: trunk/src
    depth: infinity
  - path: trunk/docs
    depth: files
`)
	fake := &svn.FakeClient{}
	ctx := context.Background()
	opts := Options{
		ConfigPath: cfgPath,
		Workdir:    workdir,
		Client:     fake,
		Logger:     newLogger(),
	}
	r1 := Apply(ctx, opts)
	if r1.Err != nil {
		t.Fatalf("first apply error: %v", r1.Err)
	}

	// Second apply: src=empty (downgrade), docs excluded (removed from config)
	cfgPath = writeConfig(t, dir, `url: svn://example.com/repo
paths:
  - path: trunk/src
    depth: empty
`)
	fake.Reset()
	opts.ConfigPath = cfgPath
	r2 := Apply(ctx, opts)
	if r2.Err != nil {
		t.Fatalf("second apply error: %v", r2.Err)
	}

	// plan.Sort groups: add/upgrade (group 0) before downgrade/exclude (group 1).
	// Within group 1, sort by depth descending then path alphabetically.
	// trunk/src (downgrade) and trunk/docs (exclude) have same depth, so alphabetical: docs before src.
	downgradeIdx := -1
	excludeIdx := -1
	for i, call := range fake.Calls {
		args := call.Args
		if len(args) >= 4 && args[0] == "update" && args[1] == "--set-depth" && args[2] == "empty" && args[3] == "trunk/src" {
			downgradeIdx = i
		}
		if len(args) >= 4 && args[0] == "update" && args[1] == "--set-depth" && args[2] == "exclude" && args[3] == "trunk/docs" {
			excludeIdx = i
		}
	}
	if downgradeIdx == -1 {
		t.Error("expected a downgrade call for trunk/src")
	}
	if excludeIdx == -1 {
		t.Error("expected an exclude call for trunk/docs")
	}
	// Both should be executed; exclude before downgrade (alphabetical within same group+depth)
	if downgradeIdx != -1 && excludeIdx != -1 && excludeIdx > downgradeIdx {
		t.Errorf("exclude (idx %d) should come before downgrade (idx %d) per sort order", excludeIdx, downgradeIdx)
	}
	// Verify plan contains both kinds
	hasDowngrade := false
	hasExclude := false
	for _, a := range r2.Plan {
		if a.Kind == plan.ActionDowngrade && a.Path == "trunk/src" {
			hasDowngrade = true
		}
		if a.Kind == plan.ActionExclude && a.Path == "trunk/docs" {
			hasExclude = true
		}
	}
	if !hasDowngrade {
		t.Error("plan should contain downgrade for trunk/src")
	}
	if !hasExclude {
		t.Error("plan should contain exclude for trunk/docs")
	}
}

func TestApply_FailureWritesHalfState(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, dir, `url: svn://example.com/repo
paths:
  - path: trunk/a
    depth: infinity
  - path: trunk/b
    depth: files
  - path: trunk/c
    depth: empty
`)
	fake := &svn.FakeClient{
		FailOn: []svn.FakeFailRule{
			{ArgsContains: []string{"trunk/c"}, Stderr: "failed on c", ExitCode: 1},
		},
	}
	ctx := context.Background()
	opts := Options{
		ConfigPath: cfgPath,
		Workdir:    workdir,
		Client:     fake,
		Logger:     newLogger(),
	}

	result := Apply(ctx, opts)
	if result.Err == nil {
		t.Fatal("expected error")
	}
	if result.FailedAction == nil {
		t.Fatal("expected FailedAction to be set")
	}

	// Check state was written with half-state (ConfigHash="")
	st, exists, err := state.Load(workdir)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if !exists {
		t.Fatal("expected state file to exist")
	}
	if st.ConfigHash != "" {
		t.Errorf("expected ConfigHash='' for half-state, got %q", st.ConfigHash)
	}
	if st.URL != "svn://example.com/repo" {
		t.Errorf("expected URL='svn://example.com/repo', got %q", st.URL)
	}
}

func TestApply_DryRun(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, dir, `url: svn://example.com/repo
paths:
  - path: trunk/src
    depth: infinity
`)
	fake := &svn.FakeClient{}
	opts := Options{
		ConfigPath: cfgPath,
		Workdir:    workdir,
		DryRun:     true,
		Client:     fake,
		Logger:     newLogger(),
	}

	result := Apply(context.Background(), opts)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if len(result.Plan) == 0 {
		t.Error("expected Plan to be non-empty")
	}
	if len(fake.Calls) != 0 {
		t.Errorf("expected no svn calls in dry run, got %d", len(fake.Calls))
	}
	if result.ExecutedCount != 0 {
		t.Errorf("expected ExecutedCount=0, got %d", result.ExecutedCount)
	}
	// State file should not exist
	_, exists, _ := state.Load(workdir)
	if exists {
		t.Error("state file should not be written in dry run")
	}
}

func TestApply_RevisionForcesUpdate(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, dir, `url: svn://example.com/repo
paths:
  - path: trunk/src
    depth: empty
`)
	fake := &svn.FakeClient{}
	ctx := context.Background()
	opts := Options{
		ConfigPath: cfgPath,
		Workdir:    workdir,
		Client:     fake,
		Logger:     newLogger(),
	}

	// First apply to establish state
	r1 := Apply(ctx, opts)
	if r1.Err != nil {
		t.Fatalf("first apply error: %v", r1.Err)
	}

	// Second apply with revision - should skip fast path
	fake.Reset()
	opts.Revision = "100"
	r2 := Apply(ctx, opts)
	if r2.Err != nil {
		t.Fatalf("second apply error: %v", r2.Err)
	}
	if r2.FastPath {
		t.Error("expected FastPath=false when revision is set")
	}

	// Should have executed UpdateRoot since config matches but revision forces update
	foundUpdate := false
	for _, call := range fake.Calls {
		if len(call.Args) >= 1 && call.Args[0] == "update" {
			foundUpdate = true
			break
		}
	}
	if !foundUpdate {
		t.Errorf("expected update call, got %v", fake.Calls)
	}
}

func TestCompute_SameAsDryRun(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, dir, `url: svn://example.com/repo
paths:
  - path: trunk/src
    depth: infinity
`)
	fake := &svn.FakeClient{}
	opts := Options{
		ConfigPath: cfgPath,
		Workdir:    workdir,
		Client:     fake,
		Logger:     newLogger(),
	}

	result, err := Compute(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Plan) == 0 {
		t.Error("expected Plan to be non-empty")
	}
	if len(fake.Calls) != 0 {
		t.Errorf("expected no svn calls in compute, got %d", len(fake.Calls))
	}
	// State should not be written
	_, exists, _ := state.Load(workdir)
	if exists {
		t.Error("state file should not be written by Compute")
	}
}

func TestCompute_URLRequired(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, dir, `paths:
  - path: trunk/src
    depth: empty
`)
	fake := &svn.FakeClient{}
	opts := Options{
		ConfigPath: cfgPath,
		Workdir:    workdir,
		Client:     fake,
		Logger:     newLogger(),
	}

	_, err := Compute(opts)
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
	if !strings.Contains(err.Error(), "url required") {
		t.Errorf("error should contain 'url required', got: %v", err)
	}
}

func TestApply_StateWrittenOnSuccess(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, dir, `url: svn://example.com/repo
paths:
  - path: trunk/src
    depth: empty
`)
	fake := &svn.FakeClient{}
	opts := Options{
		ConfigPath: cfgPath,
		Workdir:    workdir,
		Client:     fake,
		Logger:     newLogger(),
	}

	result := Apply(context.Background(), opts)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	st, exists, err := state.Load(workdir)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if !exists {
		t.Fatal("expected state file to exist after successful apply")
	}
	if st.ConfigHash == "" {
		t.Error("expected ConfigHash to be set on success")
	}
	if st.URL != "svn://example.com/repo" {
		t.Errorf("expected URL='svn://example.com/repo', got %q", st.URL)
	}
}

func TestApply_LoadConfigError(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	fake := &svn.FakeClient{}
	opts := Options{
		ConfigPath: filepath.Join(dir, "nonexistent.yaml"),
		Workdir:    workdir,
		Client:     fake,
		Logger:     newLogger(),
	}

	result := Apply(context.Background(), opts)
	if result.Err == nil {
		t.Fatal("expected error for nonexistent config")
	}
}

func TestApply_RevisionWithEmptyActions(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, dir, `url: svn://example.com/repo
paths:
  - path: trunk/src
    depth: empty
`)
	fake := &svn.FakeClient{}
	ctx := context.Background()

	// First apply to write state
	opts := Options{
		ConfigPath: cfgPath,
		Workdir:    workdir,
		Client:     fake,
		Logger:     newLogger(),
	}
	r1 := Apply(ctx, opts)
	if r1.Err != nil {
		t.Fatalf("first apply error: %v", r1.Err)
	}

	// Second apply with revision but same config -> actions empty, should do UpdateRoot
	fake.Reset()
	opts.Revision = "50"
	r2 := Apply(ctx, opts)
	if r2.Err != nil {
		t.Fatalf("second apply error: %v", r2.Err)
	}
	if r2.FastPath {
		t.Error("expected FastPath=false when revision is set")
	}

	// Should have exactly one update call (UpdateRoot)
	updateCalls := 0
	for _, call := range fake.Calls {
		if call.Args[0] == "update" {
			updateCalls++
		}
	}
	if updateCalls != 1 {
		t.Errorf("expected 1 update call for UpdateRoot, got %d; calls: %v", updateCalls, fake.Calls)
	}
	if r2.ExecutedCount != 1 {
		t.Errorf("expected ExecutedCount=1, got %d", r2.ExecutedCount)
	}
}

func TestApply_ClientFromOpts(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, dir, `url: svn://example.com/repo
paths:
  - path: trunk/src
    depth: empty
`)

	fake := &svn.FakeClient{}
	opts := Options{
		ConfigPath: cfgPath,
		Workdir:    workdir,
		Client:     fake,
		Logger:     newLogger(),
	}

	result := Apply(context.Background(), opts)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	// Verify fake was used (calls recorded)
	if len(fake.Calls) == 0 {
		t.Error("expected FakeClient to be used")
	}
}

// Ensure Result.Err and Result.FailedAction are both set on failure
func TestApply_FailedActionAndErrBothSet(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, dir, `url: svn://example.com/repo
paths:
  - path: trunk/a
    depth: infinity
  - path: trunk/b
    depth: files
`)
	fake := &svn.FakeClient{
		FailOn: []svn.FakeFailRule{
			{ArgsContains: []string{"trunk/b"}, Stderr: "permission denied", ExitCode: 1},
		},
	}
	opts := Options{
		ConfigPath: cfgPath,
		Workdir:    workdir,
		Client:     fake,
		Logger:     newLogger(),
	}

	result := Apply(context.Background(), opts)
	if result.Err == nil {
		t.Fatal("expected error")
	}
	if result.FailedAction == nil {
		t.Fatal("expected FailedAction")
	}
	if !errors.Is(result.Err, result.Err) {
		t.Error("expected Err to be set")
	}
}

// Ensure Compute works with URL override
func TestCompute_URLOverride(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "workdir")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, dir, `url: svn://example.com/repo
paths:
  - path: trunk/src
    depth: empty
`)
	fake := &svn.FakeClient{}
	opts := Options{
		ConfigPath:  cfgPath,
		Workdir:     workdir,
		URLOverride: "svn://override.com/repo",
		Client:      fake,
		Logger:      newLogger(),
	}

	result, err := Compute(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Plan) == 0 {
		t.Error("expected Plan to be non-empty")
	}
	// Verify state was NOT written
	_, exists, _ := state.Load(workdir)
	if exists {
		t.Error("Compute should not write state")
	}
}
