package cli

import (
	"os"

	"github.com/spf13/cobra"
)

const version = "0.1.0-dev"

type GlobalFlags struct {
	ConfigFile string
	Workdir    string
	Verbose    int
	Quiet      bool
	JSON       bool
	NoColor    bool
}

type exitError struct {
	Code int
	Err  error
}

func (e *exitError) Error() string { return e.Err.Error() }

func newRootCmd() *cobra.Command {
	var verbose int
	gf := &GlobalFlags{}

	cmd := &cobra.Command{
		Use:     "sparsesvn",
		Short:   "Sparse SVN working copy management tool",
		Version: version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:    cobra.NoArgs,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			gf.ConfigFile, _ = cmd.Flags().GetString("file")
			gf.Workdir, _ = cmd.Flags().GetString("workdir")
			gf.Verbose = countVerbose(cmd)
			gf.Quiet, _ = cmd.Flags().GetBool("quiet")
			gf.JSON, _ = cmd.Flags().GetBool("json")
			gf.NoColor, _ = cmd.Flags().GetBool("no-color")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.PersistentFlags().StringP("file", "f", "./sparsesvn.yaml", "config file path")
	cmd.PersistentFlags().StringP("workdir", "C", ".", "working directory")
	cmd.PersistentFlags().CountVarP(&verbose, "verbose", "v", "verbose output (can be specified multiple times)")
	cmd.PersistentFlags().BoolP("quiet", "q", false, "suppress output")
	cmd.PersistentFlags().Bool("json", false, "output in JSON format")
	cmd.PersistentFlags().Bool("no-color", false, "disable colored output")

	cmd.AddCommand(newValidateCmd(gf))
	cmd.AddCommand(newPlanCmd(gf))
	cmd.AddCommand(newStatusCmd(gf))

	return cmd
}

func countVerbose(cmd *cobra.Command) int {
	n, _ := cmd.Flags().GetCount("verbose")
	return n
}

func Execute() int {
	cmd := newRootCmd()
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	if err := cmd.Execute(); err != nil {
		if ee, ok := err.(*exitError); ok {
			return ee.Code
		}
		return 2
	}
	return 0
}
