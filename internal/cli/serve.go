package cli

import (
	"ccp/internal/config"
	"ccp/internal/logging"
	"ccp/internal/server"

	"github.com/spf13/cobra"
)

func newServeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the local proxy",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			logger, closeLog, err := logging.New(cfg.Log.Level, cfg.Log.File)
			if err != nil {
				return err
			}
			defer closeLog()
			srv, err := server.New(cfg, logger)
			if err != nil {
				return err
			}
			return srv.ListenAndServe(cmd.Context())
		},
	}
}
