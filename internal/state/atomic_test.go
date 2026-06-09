package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAtomic_Success(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	want := []byte("hello atomic\n")

	if err := writeAtomic(target, want, 0644); err != nil {
		t.Fatalf("writeAtomic: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("content mismatch: got %q want %q", got, want)
	}
}

func TestWriteAtomic_Overwrites(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")

	if err := writeAtomic(target, []byte("first"), 0644); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := writeAtomic(target, []byte("second"), 0644); err != nil {
		t.Fatalf("second write: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "second" {
		t.Fatalf("expected overwrite, got %q", got)
	}
}

func TestWriteAtomic_NoTempLeftOnSuccess(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")

	if err := writeAtomic(target, []byte("payload"), 0644); err != nil {
		t.Fatalf("writeAtomic: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.tmp.*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temp files left behind: %v", matches)
	}
}

func TestWriteAtomic_DirNotExist(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "missing-subdir", "out.txt")

	err := writeAtomic(target, []byte("x"), 0644)
	if err == nil {
		t.Fatal("expected error when parent dir missing")
	}
	if !strings.Contains(err.Error(), "missing-subdir") {
		t.Logf("error does not mention path component (informational): %v", err)
	}
}
