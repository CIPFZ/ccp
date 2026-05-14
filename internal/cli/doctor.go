package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ccp/internal/config"
	"ccp/internal/model"
	"ccp/internal/server"

	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate configuration and local environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "config: ok\n")
			for name, provider := range cfg.Providers {
				_, _ = fmt.Fprintf(out, "provider %s: type=%s base_url=%s api_key=%s\n", name, provider.Type, provider.BaseURL, config.MaskSecret(provider.ResolvedAPIKey))
			}
			for alias, target := range cfg.Aliases {
				route, err := model.Resolve(alias, cfg.Aliases)
				if err != nil {
					return fmt.Errorf("alias %s=%s: %w", alias, target, err)
				}
				if _, ok := cfg.Providers[route.Provider]; !ok {
					return fmt.Errorf("alias %s references missing provider %q", alias, route.Provider)
				}
			}
			if err := os.MkdirAll(filepath.Dir(cfg.Log.File), 0700); err != nil {
				return err
			}
			if err := server.PortAvailable(cfg.Server.Host, cfg.Server.Port); err != nil {
				if strings.Contains(err.Error(), "Only one usage") || strings.Contains(err.Error(), "address already in use") {
					return fmt.Errorf("port %s:%d is already in use", cfg.Server.Host, cfg.Server.Port)
				}
				return err
			}
			_, _ = fmt.Fprintf(out, "server: http://%s:%d available\n", cfg.Server.Host, cfg.Server.Port)
			return nil
		},
	}
}
