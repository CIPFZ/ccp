package cli

import (
	"fmt"

	"ccp/internal/config"

	"github.com/spf13/cobra"
)

func newEnvCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "env",
		Short: "Print Claude Code environment variables",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadUnresolved(opts.configPath)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "$env:ANTHROPIC_BASE_URL=\"http://%s:%d\"\n", cfg.Server.Host, cfg.Server.Port)
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "$env:ANTHROPIC_API_KEY=\"local-dev-key\"")
			return nil
		},
	}
}
