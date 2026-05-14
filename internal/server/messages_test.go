package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ccp/internal/anthropic"
	"ccp/internal/config"
	"ccp/internal/providers"
)

type fakeProvider struct {
	resp       *anthropic.MessageResponse
	streamBody string
	route      providers.Route
	req        anthropic.MessageRequest
}

func (f *fakeProvider) Messages(ctx context.Context, route providers.Route, req anthropic.MessageRequest) (*anthropic.MessageResponse, error) {
	f.route = route
	f.req = req
	return f.resp, nil
}

func (f *fakeProvider) StreamMessages(ctx context.Context, route providers.Route, req anthropic.MessageRequest) (io.ReadCloser, string, error) {
	f.route = route
	f.req = req
	return io.NopCloser(strings.NewReader(f.streamBody)), "text/event-stream", nil
}

func TestMessagesNonStreaming(t *testing.T) {
	fp := &fakeProvider{resp: &anthropic.MessageResponse{
		ID:    "msg_1",
		Type:  "message",
		Role:  "assistant",
		Model: "deepseek-v4-pro",
		Content: []anthropic.ContentBlock{
			{Type: "text", Text: "hello"},
		},
		StopReason: "end_turn",
	}}
	srv := &Server{
		cfg: &config.Config{
			Aliases: map[string]string{"sonnet": "deepseek:deepseek-v4-pro"},
		},
		logger:    slog.Default(),
		providers: map[string]providers.Provider{"deepseek": fp},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"sonnet","max_tokens":128,"messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"hello"`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
	if fp.route.Provider != "deepseek" || fp.route.Model != "deepseek-v4-pro" {
		t.Fatalf("route=%+v", fp.route)
	}
}

func TestMessagesMissingModelReturnsBadRequest(t *testing.T) {
	srv := &Server{cfg: &config.Config{}, logger: slog.Default(), providers: map[string]providers.Provider{}}
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"messages":[]}`))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestMessagesStreaming(t *testing.T) {
	fp := &fakeProvider{streamBody: "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"}
	srv := &Server{
		cfg:       &config.Config{Aliases: map[string]string{"haiku": "deepseek:deepseek-v4-flash"}},
		logger:    slog.Default(),
		providers: map[string]providers.Provider{"deepseek": fp},
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"haiku","stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("content-type=%q", got)
	}
	if !strings.Contains(rec.Body.String(), "message_stop") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestMessagesReturnsBusyWhenGlobalLimiterFull(t *testing.T) {
	fp := &fakeProvider{resp: &anthropic.MessageResponse{ID: "msg_1", Type: "message", Role: "assistant", Model: "m"}}
	global := newLimiter(1, time.Millisecond)
	release, err := global.acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	srv := &Server{
		cfg:             &config.Config{Aliases: map[string]string{"haiku": "deepseek:model"}},
		logger:          slog.Default(),
		providers:       map[string]providers.Provider{"deepseek": fp},
		globalLimiter:   global,
		providerLimiter: map[string]*limiter{"deepseek": newLimiter(1, time.Millisecond)},
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"haiku","messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
