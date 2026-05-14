package anthropicprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ccp/internal/anthropic"
	"ccp/internal/providers"
)

type Provider struct {
	client  *http.Client
	baseURL string
	apiKey  string
	headers map[string]string
}

func New(cfg providers.Config) (*Provider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("missing base url")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("missing api key")
	}
	client, err := providers.NewHTTPClient(cfg.Proxy)
	if err != nil {
		return nil, err
	}
	return &Provider{
		client:  client,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		headers: cfg.Headers,
	}, nil
}

func (p *Provider) Messages(ctx context.Context, route providers.Route, req anthropic.MessageRequest) (*anthropic.MessageResponse, error) {
	req.Model = route.Model
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.messagesURL(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	p.setHeaders(httpReq)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out anthropic.MessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) StreamMessages(ctx context.Context, route providers.Route, req anthropic.MessageRequest) (io.ReadCloser, string, error) {
	req.Model = route.Model
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.messagesURL(), bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	p.setHeaders(httpReq)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, "", fmt.Errorf("upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/event-stream"
	}
	return resp.Body, contentType, nil
}

func (p *Provider) messagesURL() string {
	if strings.HasSuffix(p.baseURL, "/v1") {
		return p.baseURL + "/messages"
	}
	return p.baseURL + "/v1/messages"
}

func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	for k, v := range p.headers {
		req.Header.Set(k, v)
	}
}
