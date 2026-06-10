package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestExecute_NoArgs(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := newRootCmd("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-C", tmpDir})

	_ = cmd.Execute()
}

func TestExecute_UnknownCommand(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := newRootCmd("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-C", tmpDir, "foobar"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown command, got nil")
	}
}

func TestGlobalFlags_Defaults(t *testing.T) {
	tmpDir := t.TempDir()

	flags := GlobalFlags{}
	cmd := newRootCmd("test")
	cmd.RunE = func(c *cobra.Command, args []string) error {
		flags.ConfigFile = c.Flag("file").Value.String()
		flags.Workdir = c.Flag("workdir").Value.String()
		flags.Verbose, _ = c.Flags().GetCount("verbose")
		flags.Quiet, _ = c.Flags().GetBool("quiet")
		flags.JSON, _ = c.Flags().GetBool("json")
		flags.NoColor, _ = c.Flags().GetBool("no-color")
		return nil
	}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"-C", tmpDir})

	_ = cmd.Execute()

	if flags.ConfigFile != "./sparsesvn.yaml" {
		t.Errorf("ConfigFile default = %q, want %q", flags.ConfigFile, "./sparsesvn.yaml")
	}
	if flags.Workdir != tmpDir {
		t.Errorf("Workdir = %q, want %q", flags.Workdir, tmpDir)
	}
	if flags.Verbose != 0 {
		t.Errorf("Verbose default = %d, want 0", flags.Verbose)
	}
	if flags.Quiet != false {
		t.Errorf("Quiet default = %v, want false", flags.Quiet)
	}
	if flags.JSON != false {
		t.Errorf("JSON default = %v, want false", flags.JSON)
	}
	if flags.NoColor != false {
		t.Errorf("NoColor default = %v, want false", flags.NoColor)
	}
}

func TestVerboseFlag_Counts(t *testing.T) {
	tmpDir := t.TempDir()

	cases := []struct {
		args []string
		want int
	}{
		{[]string{"-C", tmpDir, "-v"}, 1},
		{[]string{"-C", tmpDir, "-vv"}, 2},
		{[]string{"-C", tmpDir, "-v", "-v"}, 2},
	}
	for _, c := range cases {
		var got int
		cmd := newRootCmd("test")
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			got = countVerbose(cmd)
			return nil
		}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs(c.args)
		_ = cmd.Execute()

		if got != c.want {
			t.Errorf("args %v: Verbose = %d, want %d", c.args, got, c.want)
		}
	}
}
