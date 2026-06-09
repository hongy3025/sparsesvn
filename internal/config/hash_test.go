package config

import (
	"os"
	"path/filepath"
	"testing"
)

const emptySHA256 = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func writeTempFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return p
}

func TestHashFile_Deterministic(t *testing.T) {
	content := []byte("hello sparsesvn\n")
	p1 := writeTempFile(t, "a.txt", content)
	p2 := writeTempFile(t, "b.txt", content)

	h1, err := HashFile(p1)
	if err != nil {
		t.Fatalf("HashFile(p1) err = %v", err)
	}
	h2, err := HashFile(p2)
	if err != nil {
		t.Fatalf("HashFile(p2) err = %v", err)
	}
	if h1 != h2 {
		t.Errorf("hashes differ for identical content: %s vs %s", h1, h2)
	}
}

func TestHashFile_DifferentContent(t *testing.T) {
	p1 := writeTempFile(t, "a.txt", []byte("aaa"))
	p2 := writeTempFile(t, "b.txt", []byte("bbb"))

	h1, err := HashFile(p1)
	if err != nil {
		t.Fatalf("HashFile(p1) err = %v", err)
	}
	h2, err := HashFile(p2)
	if err != nil {
		t.Fatalf("HashFile(p2) err = %v", err)
	}
	if h1 == h2 {
		t.Errorf("hashes equal for different content: %s", h1)
	}
}

func TestHashFile_KnownVector(t *testing.T) {
	p := writeTempFile(t, "empty.txt", nil)
	got, err := HashFile(p)
	if err != nil {
		t.Fatalf("HashFile err = %v", err)
	}
	if got != emptySHA256 {
		t.Errorf("HashFile(empty) = %s, want %s", got, emptySHA256)
	}
}

func TestHashFile_NotFound(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := HashFile(missing); err == nil {
		t.Fatal("HashFile(missing) err = nil, want non-nil")
	}
}
