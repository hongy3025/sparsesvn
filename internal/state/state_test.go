package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sparsesvn/sparsesvn/internal/config"
)

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 9, 10, 30, 0, 0, time.FixedZone("CST", 8*3600))
	in := &State{
		Version:    StateVersion,
		ConfigHash: "sha256:7f3a",
		URL:        "svn://server/repo/trunk",
		AppliedAt:  now,
		Paths: []PathEntry{
			{Path: "src", Depth: config.DepthEmpty},
			{Path: "src/core", Depth: config.DepthInfinity},
		},
	}

	if err := Save(dir, in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, ok, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.Version != in.Version {
		t.Errorf("version: got %d want %d", got.Version, in.Version)
	}
	if got.ConfigHash != in.ConfigHash {
		t.Errorf("hash: got %q want %q", got.ConfigHash, in.ConfigHash)
	}
	if got.URL != in.URL {
		t.Errorf("url: got %q want %q", got.URL, in.URL)
	}
	if !got.AppliedAt.Equal(in.AppliedAt) {
		t.Errorf("applied_at: got %v want %v", got.AppliedAt, in.AppliedAt)
	}
	if len(got.Paths) != len(in.Paths) {
		t.Fatalf("paths len: got %d want %d", len(got.Paths), len(in.Paths))
	}
	for i := range got.Paths {
		if got.Paths[i] != in.Paths[i] {
			t.Errorf("paths[%d]: got %+v want %+v", i, got.Paths[i], in.Paths[i])
		}
	}
}

func TestLoad_NotFound(t *testing.T) {
	dir := t.TempDir()
	got, ok, err := Load(dir)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if ok {
		t.Error("expected ok=false")
	}
	if got != nil {
		t.Error("expected nil state")
	}
}

func TestLoad_CorruptYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".svn"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(Path(dir), []byte("not: : valid: yaml: ["), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, ok, err := Load(dir)
	if err == nil {
		t.Fatal("expected error")
	}
	if ok {
		t.Error("expected ok=false on error")
	}
	if !strings.Contains(err.Error(), "deleting the state file") {
		t.Errorf("error should mention deleting the state file, got: %v", err)
	}
}

func TestLoad_FutureVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".svn"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "version: 999\nconfig_hash: \"\"\nurl: \"\"\napplied_at: 2026-06-09T10:30:00Z\npaths: []\n"
	if err := os.WriteFile(Path(dir), []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, ok, err := Load(dir)
	if err == nil {
		t.Fatal("expected error")
	}
	if ok {
		t.Error("expected ok=false on error")
	}
	if !strings.Contains(err.Error(), "upgrade") {
		t.Errorf("error should mention upgrade, got: %v", err)
	}
}

func TestSave_PreservesOrder(t *testing.T) {
	dir := t.TempDir()
	in := &State{
		Version:    StateVersion,
		ConfigHash: "sha256:abc",
		URL:        "svn://x",
		AppliedAt:  time.Now().UTC(),
		Paths: []PathEntry{
			{Path: "zeta", Depth: config.DepthEmpty},
			{Path: "alpha", Depth: config.DepthInfinity},
			{Path: "mu", Depth: config.DepthFiles},
		},
	}
	original := make([]PathEntry, len(in.Paths))
	copy(original, in.Paths)

	if err := Save(dir, in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	for i := range in.Paths {
		if in.Paths[i] != original[i] {
			t.Errorf("Save mutated input paths at %d", i)
		}
	}

	data, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	text := string(data)
	iAlpha := strings.Index(text, "alpha")
	iMu := strings.Index(text, "mu")
	iZeta := strings.Index(text, "zeta")
	if iAlpha < 0 || iMu < 0 || iZeta < 0 {
		t.Fatalf("missing path in output:\n%s", text)
	}
	if !(iAlpha < iMu && iMu < iZeta) {
		t.Errorf("paths not in lexicographic order:\n%s", text)
	}
	if !strings.Contains(text, "# sparsesvn state file - DO NOT EDIT MANUALLY") {
		t.Error("missing header comment")
	}
}

func TestSave_CreatesStateDir(t *testing.T) {
	dir := t.TempDir()
	if _, err := os.Stat(filepath.Join(dir, ".svn")); !os.IsNotExist(err) {
		t.Fatalf("precondition: .svn should not exist, got err=%v", err)
	}

	s := &State{
		Version:    StateVersion,
		ConfigHash: "",
		URL:        "svn://x",
		AppliedAt:  time.Now().UTC(),
		Paths:      nil,
	}
	if err := Save(dir, s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(Path(dir)); err != nil {
		t.Errorf("state file not created: %v", err)
	}
}

func TestSave_UTCConversion(t *testing.T) {
	dir := t.TempDir()
	loc := time.FixedZone("CST", 8*3600)
	local := time.Date(2026, 6, 9, 18, 30, 0, 0, loc)
	s := &State{
		Version:    StateVersion,
		ConfigHash: "sha256:x",
		URL:        "svn://x",
		AppliedAt:  local,
		Paths:      nil,
	}

	if err := Save(dir, s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !s.AppliedAt.Equal(local) || s.AppliedAt.Location().String() != loc.String() {
		t.Error("Save mutated input AppliedAt")
	}

	data, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "2026-06-09T10:30:00Z") {
		t.Errorf("expected UTC timestamp in file, got:\n%s", data)
	}
}
