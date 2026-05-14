package cli

import "github.com/spf13/cobra"

type options struct {
	configPath string
}

var opts options

func Execute() error {
	root := &cobra.Command{
		Use:   "ccp",
		Short: "Local Anthropic-compatible proxy for Claude Code",
	}
	root.PersistentFlags().StringVar(&opts.configPath, "config", "", "config file path")
	root.AddCommand(newServeCommand())
	root.AddCommand(newDoctorCommand())
	root.AddCommand(newEnvCommand())
	root.AddCommand(newTestCommand())
	return root.Execute()
}
