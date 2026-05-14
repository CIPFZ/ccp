package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"ccp/internal/config"
	"ccp/internal/providers"
	anthropicprovider "ccp/internal/providers/anthropic"
	openaiprovider "ccp/internal/providers/openai"
)

type Server struct {
	cfg             *config.Config
	logger          *slog.Logger
	providers       map[string]providers.Provider
	globalLimiter   *limiter
	providerLimiter map[string]*limiter
}

func New(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	registry := map[string]providers.Provider{}
	providerLimiters := map[string]*limiter{}
	for name, providerCfg := range cfg.Providers {
		pcfg := providers.Config{
			Name:    name,
			Type:    providerCfg.Type,
			BaseURL: providerCfg.BaseURL,
			APIKey:  providerCfg.ResolvedAPIKey,
			Proxy: providers.ProxyConfig{
				Enabled: providerCfg.Proxy.Enabled,
				URL:     providerCfg.Proxy.URL,
			},
			Headers: providerCfg.Headers,
		}
		switch providerCfg.Type {
		case "anthropic-compatible":
			p, err := anthropicprovider.New(pcfg)
			if err != nil {
				return nil, fmt.Errorf("provider %q: %w", name, err)
			}
			registry[name] = p
		case "openai-compatible":
			p, err := openaiprovider.New(pcfg)
			if err != nil {
				return nil, fmt.Errorf("provider %q: %w", name, err)
			}
			registry[name] = p
		default:
			return nil, fmt.Errorf("provider %q: unknown type %q", name, providerCfg.Type)
		}
		providerLimiters[name] = newLimiter(providerCfg.MaxConcurrentRequests, 2*time.Second)
	}
	return &Server{
		cfg:             cfg,
		logger:          logger,
		providers:       registry,
		globalLimiter:   newLimiter(cfg.Server.MaxConcurrentRequests, 2*time.Second),
		providerLimiter: providerLimiters,
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /v1/messages", s.handleMessages)
	return mux
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("listening", "addr", addr)
		errCh <- httpServer.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func PortAvailable(host string, port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return err
	}
	return ln.Close()
}
