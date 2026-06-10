package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hongy3025/sparsesvn/internal/executor"
	"github.com/spf13/cobra"
)

func newPlanCmd(gf *GlobalFlags) *cobra.Command {
	var urlOverride string
	var revision string

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Show planned actions (dry-run diff)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 显示上下文信息（除非 quiet 模式）
			if !gf.Quiet {
				displayContext(cmd.ErrOrStderr(), gf.Workdir, gf.ResolvedURL, gf.ConfigFile)
				fmt.Fprintln(cmd.ErrOrStderr()) // 空行分隔
			}

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

			if gf.JSON {
				url := urlOverride
				if url == "" {
					url = result.StateAfter.URL
				}
				pj := BuildPlanJSON(url, result.Plan)
				data, err := json.MarshalIndent(pj, "", "  ")
				if err != nil {
					return &exitError{Code: 1, Err: err}
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), FormatPlan(result.Plan))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&urlOverride, "url", "", "override URL from config")
	cmd.Flags().StringVarP(&revision, "revision", "r", "", "revision to use")

	return cmd
}
