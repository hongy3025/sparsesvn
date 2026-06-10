//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2E_PlanText(t *testing.T) {
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

	stdout, stderr, code := RunCLI(t, []string{"plan", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("plan exit=%d stderr=%s", code, stderr)
	}

	if !strings.Contains(stdout, "Plan: 3 actions") {
		t.Errorf("expected 'Plan: 3 actions' in stdout, got %q", stdout)
	}
}

func TestE2E_PlanJSON(t *testing.T) {
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

	stdout, stderr, code := RunCLI(t, []string{"plan", "-f", cfgPath, "-C", workdir, "--json"}, "")
	if code != 0 {
		t.Fatalf("plan --json exit=%d stderr=%s", code, stderr)
	}

	var pj struct {
		Url     string `json:"url"`
		Actions []struct {
			Kind string `json:"kind"`
			Path string `json:"path"`
		} `json:"actions"`
		Summary struct {
			Total int `json:"total"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &pj); err != nil {
		t.Fatalf("parse plan JSON: %v\nraw: %s", err, stdout)
	}

	if len(pj.Actions) != 3 {
		t.Errorf("expected 3 actions, got %d: %+v", len(pj.Actions), pj.Actions)
	}
	if pj.Summary.Total != 3 {
		t.Errorf("expected summary.total=3, got %d", pj.Summary.Total)
	}
}

func TestE2E_StatusInSync(t *testing.T) {
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
		t.Fatalf("apply exit=%d stderr=%s", code, stderr)
	}

	stdout, stderr, code := RunCLI(t, []string{"status", "-f", cfgPath, "-C", workdir}, "")
	if code != 0 {
		t.Fatalf("status exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	if !strings.Contains(stdout, "in sync") {
		t.Errorf("expected 'in sync' in stdout, got %q", stdout)
	}
}

func TestE2E_StatusHasDiff(t *testing.T) {
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
		t.Fatalf("apply exit=%d stderr=%s", code, stderr)
	}

	yaml2 := fmt.Sprintf(`url: %s/trunk
paths:
  - path: src/core
    depth: infinity
  - path: docs
    depth: files
`, repo.URL)
	os.WriteFile(cfgPath, []byte(yaml2), 0644)

	_, _, code = RunCLI(t, []string{"status", "-f", cfgPath, "-C", workdir}, "")
	if code != 1 {
		t.Errorf("expected status exit 1 when diff exists, got %d", code)
	}
}

func TestE2E_ValidateOK(t *testing.T) {
	RequireSvnBinary(t)
	workdir := t.TempDir()

	yaml := `url: file:///tmp/repo/trunk
paths:
  - path: src
    depth: infinity
`
	cfgPath := filepath.Join(workdir, "sparsesvn.yaml")
	os.WriteFile(cfgPath, []byte(yaml), 0644)

	_, _, code := RunCLI(t, []string{"validate", "-f", cfgPath}, "")
	if code != 0 {
		t.Errorf("expected validate exit 0, got %d", code)
	}
}

func TestE2E_ValidateBadPath(t *testing.T) {
	RequireSvnBinary(t)
	workdir := t.TempDir()

	yaml := `url: file:///tmp/repo/trunk
paths:
  - path: /abs
    depth: infinity
`
	cfgPath := filepath.Join(workdir, "sparsesvn.yaml")
	os.WriteFile(cfgPath, []byte(yaml), 0644)

	_, stderr, code := RunCLI(t, []string{"validate", "-f", cfgPath}, "")
	if code != 2 {
		t.Errorf("expected validate exit 2, got %d", code)
	}

	if !strings.Contains(stderr, "must not start with") {
		t.Errorf("expected 'must not start with' in stderr, got %q", stderr)
	}
}
