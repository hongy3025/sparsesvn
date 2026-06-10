//go:build integration

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", path)
	}
}

func assertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected file to NOT exist: %s", path)
	}
}

func TestE2E_FreshApply(t *testing.T) {
	RequireSvnBinary(t)
	repo := CreateTestRepo(t)
	workdir := t.TempDir()

	yaml := fmt.Sprintf(`url: %s/trunk
paths:
  - path: src/core
    depth: infinity
  - path: docs
    depth: files
`, repo.URL)
	cfgPath := filepath.Join(workdir, "sparsesvn.yaml")
	os.WriteFile(cfgPath, []byte(yaml), 0644)

	stdout, stderr, code := RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("apply exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	assertFileExists(t, filepath.Join(workdir, "src/core/main.c"))
	assertFileExists(t, filepath.Join(workdir, "docs/readme.md"))
	assertFileNotExists(t, filepath.Join(workdir, "src/utils/helper.c"))
	assertFileExists(t, filepath.Join(workdir, ".svn/sparsesvn.state.yaml"))
}

func TestE2E_FastPath(t *testing.T) {
	RequireSvnBinary(t)
	repo := CreateTestRepo(t)
	workdir := t.TempDir()

	yaml := fmt.Sprintf(`url: %s/trunk
paths:
  - path: src/core
    depth: infinity
`, repo.URL)
	cfgPath := filepath.Join(workdir, "sparsesvn.yaml")
	os.WriteFile(cfgPath, []byte(yaml), 0644)

	_, stderr, code := RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("first apply exit=%d stderr=%s", code, stderr)
	}

	statePath := filepath.Join(workdir, ".svn/sparsesvn.state.yaml")
	info1, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("stat state after first apply: %v", err)
	}

	stdout2, stderr2, code2 := RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code2 != 0 {
		t.Fatalf("second apply exit=%d stderr=%s", code2, stderr2)
	}

	if !strings.Contains(stdout2, "Already in sync") {
		t.Errorf("expected 'Already in sync' in stdout, got %q", stdout2)
	}

	info2, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("stat state after second apply: %v", err)
	}
	if info1.ModTime() != info2.ModTime() {
		t.Errorf("state file mtime changed on fast path (was %v, now %v)", info1.ModTime(), info2.ModTime())
	}
}

func TestE2E_AddPath(t *testing.T) {
	RequireSvnBinary(t)
	repo := CreateTestRepo(t)
	workdir := t.TempDir()

	yaml1 := fmt.Sprintf(`url: %s/trunk
paths:
  - path: src/core
    depth: infinity
`, repo.URL)
	cfgPath := filepath.Join(workdir, "sparsesvn.yaml")
	os.WriteFile(cfgPath, []byte(yaml1), 0644)

	_, stderr, code := RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("first apply exit=%d stderr=%s", code, stderr)
	}

	assertFileNotExists(t, filepath.Join(workdir, "docs/readme.md"))

	yaml2 := fmt.Sprintf(`url: %s/trunk
paths:
  - path: src/core
    depth: infinity
  - path: docs
    depth: files
`, repo.URL)
	os.WriteFile(cfgPath, []byte(yaml2), 0644)

	stdout, stderr, code := RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("second apply exit=%d stderr=%s", code, stderr)
	}

	if !strings.Contains(stdout, "Applied") {
		t.Errorf("expected 'Applied' in stdout, got %q", stdout)
	}
	assertFileExists(t, filepath.Join(workdir, "docs/readme.md"))
}

func TestE2E_UpgradeDepth(t *testing.T) {
	RequireSvnBinary(t)
	repo := CreateTestRepo(t)
	workdir := t.TempDir()

	yaml1 := fmt.Sprintf(`url: %s/trunk
paths:
  - path: docs
    depth: empty
`, repo.URL)
	cfgPath := filepath.Join(workdir, "sparsesvn.yaml")
	os.WriteFile(cfgPath, []byte(yaml1), 0644)

	_, stderr, code := RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("first apply exit=%d stderr=%s", code, stderr)
	}

	assertFileNotExists(t, filepath.Join(workdir, "docs/readme.md"))

	yaml2 := fmt.Sprintf(`url: %s/trunk
paths:
  - path: docs
    depth: files
`, repo.URL)
	os.WriteFile(cfgPath, []byte(yaml2), 0644)

	_, stderr, code = RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("second apply exit=%d stderr=%s", code, stderr)
	}

	assertFileExists(t, filepath.Join(workdir, "docs/readme.md"))
}

func TestE2E_DowngradeDepth(t *testing.T) {
	RequireSvnBinary(t)
	repo := CreateTestRepo(t)
	workdir := t.TempDir()

	yaml1 := fmt.Sprintf(`url: %s/trunk
paths:
  - path: src/core
    depth: infinity
`, repo.URL)
	cfgPath := filepath.Join(workdir, "sparsesvn.yaml")
	os.WriteFile(cfgPath, []byte(yaml1), 0644)

	_, stderr, code := RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("first apply exit=%d stderr=%s", code, stderr)
	}

	assertFileExists(t, filepath.Join(workdir, "src/core/main.c"))

	yaml2 := fmt.Sprintf(`url: %s/trunk
paths:
  - path: src/core
    depth: empty
`, repo.URL)
	os.WriteFile(cfgPath, []byte(yaml2), 0644)

	_, stderr, code = RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("second apply exit=%d stderr=%s", code, stderr)
	}

	assertFileNotExists(t, filepath.Join(workdir, "src/core/main.c"))
}

func TestE2E_ExcludePath(t *testing.T) {
	RequireSvnBinary(t)
	repo := CreateTestRepo(t)
	workdir := t.TempDir()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "trunk", "tmp", "x.txt")
	os.MkdirAll(filepath.Dir(tmpFile), 0o755)
	os.WriteFile(tmpFile, []byte("tmp file\n"), 0644)

	cmd := exec.Command("svn", "import", filepath.Join(tmpDir, "trunk"), repo.URL+"/trunk", "-m", "add tmp")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("svn import tmp: %v\n%s", err, out)
	}

	yaml1 := fmt.Sprintf(`url: %s/trunk
paths:
  - path: src/core
    depth: infinity
  - path: tmp
    depth: infinity
`, repo.URL)
	cfgPath := filepath.Join(workdir, "sparsesvn.yaml")
	os.WriteFile(cfgPath, []byte(yaml1), 0644)

	_, stderr, code := RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("first apply exit=%d stderr=%s", code, stderr)
	}

	assertFileExists(t, filepath.Join(workdir, "tmp", "x.txt"))

	yaml2 := fmt.Sprintf(`url: %s/trunk
paths:
  - path: src/core
    depth: infinity
`, repo.URL)
	os.WriteFile(cfgPath, []byte(yaml2), 0644)

	_, stderr, code = RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("second apply exit=%d stderr=%s", code, stderr)
	}

	assertFileNotExists(t, filepath.Join(workdir, "tmp"))
}

func TestE2E_URLMismatch(t *testing.T) {
	RequireSvnBinary(t)
	repo1 := CreateTestRepo(t)
	repo2 := CreateTestRepo(t)
	workdir := t.TempDir()

	yaml1 := fmt.Sprintf(`url: %s/trunk
paths:
  - path: src/core
    depth: infinity
`, repo1.URL)
	cfgPath := filepath.Join(workdir, "sparsesvn.yaml")
	os.WriteFile(cfgPath, []byte(yaml1), 0644)

	_, stderr, code := RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("first apply exit=%d stderr=%s", code, stderr)
	}

	yaml2 := fmt.Sprintf(`url: %s/trunk
paths:
  - path: src/core
    depth: infinity
`, repo2.URL)
	os.WriteFile(cfgPath, []byte(yaml2), 0644)

	_, stderr, code = RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d stderr=%s", code, stderr)
	}

	if !strings.Contains(stderr, "url mismatch") {
		t.Errorf("expected 'url mismatch' in stderr, got %q", stderr)
	}
}

func TestE2E_StateMissing_FullRebuild(t *testing.T) {
	RequireSvnBinary(t)
	repo := CreateTestRepo(t)
	workdir := t.TempDir()

	yaml := fmt.Sprintf(`url: %s/trunk
paths:
  - path: src/core
    depth: infinity
  - path: docs
    depth: files
`, repo.URL)
	cfgPath := filepath.Join(workdir, "sparsesvn.yaml")
	os.WriteFile(cfgPath, []byte(yaml), 0644)

	_, stderr, code := RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("first apply exit=%d stderr=%s", code, stderr)
	}

	statePath := filepath.Join(workdir, ".svn/sparsesvn.state.yaml")
	os.Remove(statePath)

	stdout, stderr, code := RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("second apply exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	assertFileExists(t, filepath.Join(workdir, "src/core/main.c"))
	assertFileExists(t, filepath.Join(workdir, "docs/readme.md"))
	assertFileExists(t, statePath)
}

func TestE2E_SvnFailure_HalfState(t *testing.T) {
	RequireSvnBinary(t)
	repo := CreateTestRepo(t)
	workdir := t.TempDir()

	yaml := fmt.Sprintf(`url: %s/trunk
paths:
  - path: src/core
    depth: infinity
  - path: nonexistent/dir
    depth: infinity
`, repo.URL)
	cfgPath := filepath.Join(workdir, "sparsesvn.yaml")
	os.WriteFile(cfgPath, []byte(yaml), 0644)

	_, stderr, code := RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 3 {
		t.Fatalf("expected exit 3, got %d stderr=%s", code, stderr)
	}

	assertFileExists(t, filepath.Join(workdir, ".svn/sparsesvn.state.yaml"))

	st, err := os.ReadFile(filepath.Join(workdir, ".svn/sparsesvn.state.yaml"))
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	stateContent := string(st)
	if !strings.Contains(stateContent, "config_hash: \"\"") {
		t.Errorf("state config_hash should be empty, got:\n%s", stateContent)
	}
	if !strings.Contains(stateContent, "src") {
		t.Errorf("state should contain src, got:\n%s", stateContent)
	}
	if strings.Contains(stateContent, "nonexistent/dir") {
		t.Errorf("state should NOT contain nonexistent/dir (failed action), got:\n%s", stateContent)
	}
}

func TestE2E_RevisionAlignment(t *testing.T) {
	RequireSvnBinary(t)
	repo := CreateTestRepo(t)
	workdir := t.TempDir()

	yaml := fmt.Sprintf(`url: %s/trunk
paths:
  - path: src/core
    depth: infinity
`, repo.URL)
	cfgPath := filepath.Join(workdir, "sparsesvn.yaml")
	os.WriteFile(cfgPath, []byte(yaml), 0644)

	_, stderr, code := RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("first apply exit=%d stderr=%s", code, stderr)
	}

	stdout, stderr, code := RunCLI(t, []string{"apply", "-f", cfgPath, "-C", workdir, "-r", "1"}, "")
	if code != 0 {
		t.Fatalf("apply -r 1 exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	infoOut, err := exec.Command("svn", "info", "--show-item", "revision", workdir).CombinedOutput()
	if err != nil {
		t.Fatalf("svn info: %v\n%s", err, infoOut)
	}
	rev := strings.TrimSpace(string(infoOut))
	if rev != "1" {
		t.Errorf("expected revision 1, got %q", rev)
	}
}
