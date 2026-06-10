package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/sparsesvn/sparsesvn/internal/executor"
	"github.com/sparsesvn/sparsesvn/internal/logx"
	"github.com/sparsesvn/sparsesvn/internal/svn"
	"github.com/spf13/cobra"
)

type ApplyFlags struct {
	URL      string
	Revision string
	DryRun   bool
}

func newApplyCmd(gf *GlobalFlags) *cobra.Command {
	var flags ApplyFlags

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply config to working copy (checkout + set-depth)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			code := runApply(
				context.Background(),
				gf,
				flags,
				svn.NewExecClient(),
				cmd.OutOrStdout(),
				cmd.ErrOrStderr(),
			)
			if code != 0 {
				return &exitError{Code: code, Err: fmt.Errorf("apply exited with code %d", code)}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.URL, "url", "", "override URL from config")
	cmd.Flags().StringVarP(&flags.Revision, "revision", "r", "", "revision to use")
	cmd.Flags().BoolVar(&flags.DryRun, "dry-run", false, "show plan without executing")

	return cmd
}

func runApply(ctx context.Context, gf *GlobalFlags, applyFlags ApplyFlags, client svn.Client, out io.Writer, errOut io.Writer) int {
	level := logx.LevelNormal
	if gf.Verbose >= 2 {
		level = logx.LevelDebug
	} else if gf.Verbose == 1 {
		level = logx.LevelVerbose
	}
	if gf.Quiet {
		level = logx.LevelQuiet
	}
	logger := logx.New(errOut, level, gf.JSON)

	opts := executor.Options{
		ConfigPath:  gf.ConfigFile,
		Workdir:     gf.Workdir,
		URLOverride: applyFlags.URL,
		Revision:    applyFlags.Revision,
		Client:      client,
		Logger:      logger,
	}
	if applyFlags.DryRun {
		opts.DryRun = true
	}

	start := time.Now()
	result := executor.Apply(ctx, opts)
	elapsed := time.Since(start)

	if result.FastPath {
		fmt.Fprintln(out, "Already in sync")
		return 0
	}

	if result.Err != nil {
		if result.FailedAction != nil {
			fmt.Fprintf(errOut, "Failed: %s %s\n", result.FailedAction.Kind, result.FailedAction.Path)
			fmt.Fprintln(errOut, result.Err)
			return 3
		}
		errMsg := result.Err.Error()
		if strings.Contains(errMsg, "url mismatch") || strings.Contains(errMsg, "url required") || strings.Contains(errMsg, "load config") {
			fmt.Fprintln(errOut, result.Err)
			return 2
		}
		fmt.Fprintln(errOut, result.Err)
		return 1
	}

	if applyFlags.DryRun {
		fmt.Fprintln(out, FormatPlan(result.Plan))
		return 0
	}

	fmt.Fprintf(out, "Applied %d actions in %s\n", result.ExecutedCount, formatDuration(elapsed))
	return 0
}

func formatDuration(d time.Duration) string {
	ms := float64(d) / float64(time.Millisecond)
	if ms < 1000 {
		return fmt.Sprintf("%.1fms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(d)/float64(time.Second))
}
