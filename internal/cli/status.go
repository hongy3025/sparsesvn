package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/hongy3025/sparsesvn/internal/executor"
	"github.com/hongy3025/sparsesvn/internal/svn"
	"github.com/spf13/cobra"
)

type StatusJSON struct {
	PlanJSON
	InSync bool `json:"in_sync"`
}

type StatusFlags struct {
	URL      string
	Revision string
}

func newStatusCmd(gf *GlobalFlags) *cobra.Command {
	var flags StatusFlags

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check if working copy is in sync with config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			code := runStatus(
				context.Background(),
				gf,
				flags,
				svn.NewExecClient(),
				cmd.OutOrStdout(),
			)
			if code != 0 {
				return &exitError{Code: code, Err: fmt.Errorf("status exited with code %d", code)}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.URL, "url", "", "override URL from config")
	cmd.Flags().StringVarP(&flags.Revision, "revision", "r", "", "revision to use")

	return cmd
}

func runStatus(ctx context.Context, gf *GlobalFlags, flags StatusFlags, client svn.Client, out io.Writer) int {
	opts := executor.Options{
		ConfigPath:  gf.ConfigFile,
		Workdir:     gf.Workdir,
		URLOverride: flags.URL,
		Revision:    flags.Revision,
		Client:      client,
	}

	result, err := executor.Compute(ctx, opts)
	if err != nil {
		return 2
	}

	inSync := len(result.Plan) == 0

	if gf.JSON {
		url := flags.URL
		if url == "" {
			url = result.StateAfter.URL
		}
		sj := StatusJSON{
			PlanJSON: BuildPlanJSON(url, result.Plan),
			InSync:   inSync,
		}
		data, err := json.MarshalIndent(sj, "", "  ")
		if err != nil {
			return 1
		}
		fmt.Fprintln(out, string(data))
	} else {
		if inSync {
			fmt.Fprintln(out, "in sync")
		} else {
			fmt.Fprintln(out, FormatPlan(result.Plan))
		}
	}

	if !inSync {
		return 1
	}
	return 0
}
