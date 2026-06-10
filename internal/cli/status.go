package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hongy3025/sparsesvn/internal/executor"
	"github.com/spf13/cobra"
)

type StatusJSON struct {
	PlanJSON
	InSync bool `json:"in_sync"`
}

func newStatusCmd(gf *GlobalFlags) *cobra.Command {
	var urlOverride string
	var revision string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check if working copy is in sync with config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := executor.Options{
				ConfigPath:  gf.ConfigFile,
				Workdir:     gf.Workdir,
				URLOverride: urlOverride,
				Revision:    revision,
			}

			result, err := executor.Compute(context.Background(), opts)
			if err != nil {
				return &exitError{Code: 2, Err: err}
			}

			inSync := len(result.Plan) == 0

			if gf.JSON {
				url := urlOverride
				if url == "" {
					url = result.StateAfter.URL
				}
				sj := StatusJSON{
					PlanJSON: BuildPlanJSON(url, result.Plan),
					InSync:   inSync,
				}
				data, err := json.MarshalIndent(sj, "", "  ")
				if err != nil {
					return &exitError{Code: 1, Err: err}
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				if inSync {
					fmt.Fprintln(cmd.OutOrStdout(), "in sync")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), FormatPlan(result.Plan))
				}
			}

			if !inSync {
				return &exitError{Code: 1, Err: fmt.Errorf("working copy not in sync")}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&urlOverride, "url", "", "override URL from config")
	cmd.Flags().StringVarP(&revision, "revision", "r", "", "revision to use")

	return cmd
}
