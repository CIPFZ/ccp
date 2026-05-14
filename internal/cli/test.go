package cli

import (
	"context"
	"fmt"
	"time"

	"ccp/internal/anthropic"
	"ccp/internal/config"
	"ccp/internal/model"
	"ccp/internal/providers"
	anthropicprovider "ccp/internal/providers/anthropic"
	openaiprovider "ccp/internal/providers/openai"

	"github.com/spf13/cobra"
)

func newTestCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "test <alias> <prompt>",
		Short: "Send a test prompt through a configured model alias",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			route, err := model.Resolve(args[0], cfg.Aliases)
			if err != nil {
				return err
			}
			pcfg, ok := cfg.Providers[route.Provider]
			if !ok {
				return fmt.Errorf("provider %q not configured", route.Provider)
			}
			providerCfg := providers.Config{
				Name:    route.Provider,
				Type:    pcfg.Type,
				BaseURL: pcfg.BaseURL,
				APIKey:  pcfg.ResolvedAPIKey,
				Proxy: providers.ProxyConfig{
					Enabled: pcfg.Proxy.Enabled,
					URL:     pcfg.Proxy.URL,
				},
				Headers: pcfg.Headers,
			}
			var provider providers.Provider
			switch pcfg.Type {
			case "anthropic-compatible":
				provider, err = anthropicprovider.New(providerCfg)
			case "openai-compatible":
				provider, err = openaiprovider.New(providerCfg)
			default:
				err = fmt.Errorf("unknown provider type %q", pcfg.Type)
			}
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			resp, err := provider.Messages(ctx, providers.Route(route), anthropic.MessageRequest{
				Model:     args[0],
				MaxTokens: 512,
				Messages:  []anthropic.Message{{Role: "user", Content: args[1]}},
			})
			if err != nil {
				return err
			}
			for _, block := range resp.Content {
				if block.Type == "text" && block.Text != "" {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), block.Text)
				}
			}
			return nil
		},
	}
}
