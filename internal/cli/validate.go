package cli

import (
	"fmt"

	"github.com/sparsesvn/sparsesvn/internal/config"
	"github.com/spf13/cobra"
)

func newValidateCmd(gf *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := gf.ConfigFile
			if _, err := config.Load(path); err != nil {
				return &exitError{Code: 2, Err: err}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "OK: %s is valid\n", path)
			return nil
		},
	}
}
