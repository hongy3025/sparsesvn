//go:build integration

package integration

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCreateTestRepo_HasFiles(t *testing.T) {
	RequireSvnBinary(t)
	repo := CreateTestRepo(t)

	out, err := exec.Command("svn", "ls", "-R", repo.URL).CombinedOutput()
	if err != nil {
		t.Fatalf("svn ls failed: %v\n%s", err, out)
	}

	want := []string{
		"trunk/",
		"trunk/src/",
		"trunk/src/core/",
		"trunk/src/core/main.c",
		"trunk/src/core/util.c",
		"trunk/src/utils/",
		"trunk/src/utils/helper.c",
		"trunk/docs/",
		"trunk/docs/readme.md",
		"trunk/tests/",
		"trunk/tests/unit/",
		"trunk/tests/unit/test_main.c",
		"trunk/tests/integration/",
		"trunk/tests/integration/test_api.c",
	}

	ls := string(out)
	for _, w := range want {
		if !strings.Contains(ls, w) {
			t.Errorf("svn ls output missing %q\nfull output:\n%s", w, ls)
		}
	}
}

func TestBuildBinary_Exists(t *testing.T) {
	bin := BuildBinary(t)

	cmd := exec.Command(bin, "--help")
	if err := cmd.Run(); err != nil {
		t.Fatalf("binary not executable: %v", err)
	}
}

func TestRunCLI_Version(t *testing.T) {
	stdout, _, exitCode := RunCLI(t, []string{"--version"}, "")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "0.1.0-dev") {
		t.Errorf("expected version in stdout, got %q", stdout)
	}
}
