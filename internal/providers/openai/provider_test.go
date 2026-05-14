package openaiprovider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ccp/internal/anthropic"
	"ccp/internal/providers"
)

func TestProviderStreamMessagesConvertsToolCalls(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"Read","arguments":"{\"file_"}}]}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"path\":\"a.txt\"}"}}]}}]}`,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	defer upstream.Close()

	p, err := New(providers.Config{
		BaseURL: upstream.URL,
		APIKey:  "sk-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	body, contentType, err := p.StreamMessages(context.Background(), providers.Route{Model: "gpt-5.5"}, anthropic.MessageRequest{
		Messages: []anthropic.Message{{Role: "user", Content: "use tool"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	if contentType != "text/event-stream" {
		t.Fatalf("contentType=%q", contentType)
	}
	b, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.Contains(got, `"type":"tool_use"`) || !strings.Contains(got, `"type":"input_json_delta"`) || !strings.Contains(got, `"stop_reason":"tool_use"`) {
		t.Fatalf("stream missing tool events:\n%s", got)
	}
}

func TestProviderMessagesAcceptsSSEWrappedNonStreamingResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, `data: {"id":"chatcmpl_1","choices":[{"message":{"role":"assistant","content":"Hi from SSE wrapper"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`+"\n\n")
	}))
	defer upstream.Close()

	p, err := New(providers.Config{
		BaseURL: upstream.URL,
		APIKey:  "sk-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := p.Messages(context.Background(), providers.Route{Model: "gpt-5.5"}, anthropic.MessageRequest{
		Messages: []anthropic.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content[0].Text != "Hi from SSE wrapper" {
		t.Fatalf("resp=%+v", resp)
	}
	if resp.Usage.InputTokens != 3 || resp.Usage.OutputTokens != 4 {
		t.Fatalf("usage=%+v", resp.Usage)
	}
}
