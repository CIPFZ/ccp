package providers

import (
	"context"
	"io"

	"ccp/internal/anthropic"
)

type Route struct {
	Alias    string
	Provider string
	Model    string
}

type Provider interface {
	Messages(ctx context.Context, route Route, req anthropic.MessageRequest) (*anthropic.MessageResponse, error)
	StreamMessages(ctx context.Context, route Route, req anthropic.MessageRequest) (io.ReadCloser, string, error)
}

type Config struct {
	Name    string
	Type    string
	BaseURL string
	APIKey  string
	Proxy   ProxyConfig
	Headers map[string]string
}

type ProxyConfig struct {
	Enabled bool
	URL     string
}
