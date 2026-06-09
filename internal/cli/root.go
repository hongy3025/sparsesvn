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

	cmd := &cobra.Command{
		Use:     "sparsesvn",
		Short:   "Sparse SVN working copy management tool",
		Version: version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:    cobra.NoArgs,
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
