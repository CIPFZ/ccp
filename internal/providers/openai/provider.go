package openaiprovider

import (
	"bufio"
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
	req.Stream = false
	chat, err := buildChatRequest(route.Model, req)
	if err != nil {
		return nil, err
	}
	var resp chatResponse
	if err := p.doJSON(ctx, chat, &resp); err != nil {
		return nil, err
	}
	return convertChatResponse(route.Model, resp)
}

func (p *Provider) StreamMessages(ctx context.Context, route providers.Route, req anthropic.MessageRequest) (io.ReadCloser, string, error) {
	req.Stream = true
	chat, err := buildChatRequest(route.Model, req)
	if err != nil {
		return nil, "", err
	}
	body, err := json.Marshal(chat)
	if err != nil {
		return nil, "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.chatURL(), bytes.NewReader(body))
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
	pr, pw := io.Pipe()
	go func() {
		defer resp.Body.Close()
		defer pw.Close()
		convertOpenAIStream(resp.Body, pw, route.Model)
	}()
	return pr, "text/event-stream", nil
}

func (p *Provider) doJSON(ctx context.Context, in any, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.chatURL(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	p.setHeaders(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, out); err == nil {
		return nil
	}
	return decodeSSEDataPayload(b, out)
}

func (p *Provider) chatURL() string {
	if strings.HasSuffix(p.baseURL, "/v1") {
		return p.baseURL + "/chat/completions"
	}
	return p.baseURL + "/v1/chat/completions"
}

func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range p.headers {
		req.Header.Set(k, v)
	}
}

func convertOpenAIStream(r io.Reader, w io.Writer, model string) {
	writeSSE(w, "message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            "msg_ccp_stream",
			"type":          "message",
			"role":          "assistant",
			"model":         model,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         map[string]any{"input_tokens": 0, "output_tokens": 0},
		},
	})
	textStarted := false
	toolStarted := map[int]bool{}
	blockOpen := false
	stopReason := "end_turn"
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk chatResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil || len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		for _, call := range delta.ToolCalls {
			idx := call.Index
			if !toolStarted[idx] {
				id := call.ID
				if id == "" {
					id = fmt.Sprintf("call_%d", idx)
				}
				writeSSE(w, "content_block_start", map[string]any{
					"type":  "content_block_start",
					"index": idx,
					"content_block": map[string]any{
						"type":  "tool_use",
						"id":    id,
						"name":  call.Function.Name,
						"input": map[string]any{},
					},
				})
				toolStarted[idx] = true
				blockOpen = true
			}
			if call.Function.Arguments != "" {
				writeSSE(w, "content_block_delta", map[string]any{
					"type":  "content_block_delta",
					"index": idx,
					"delta": map[string]any{
						"type":         "input_json_delta",
						"partial_json": call.Function.Arguments,
					},
				})
			}
			stopReason = "tool_use"
		}
		text := delta.Content
		if text != "" {
			if !textStarted {
				writeSSE(w, "content_block_start", map[string]any{
					"type":          "content_block_start",
					"index":         0,
					"content_block": map[string]any{"type": "text", "text": ""},
				})
				textStarted = true
				blockOpen = true
			}
			writeSSE(w, "content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]any{"type": "text_delta", "text": text},
			})
		}
	}
	if blockOpen {
		if len(toolStarted) > 0 {
			for idx := range toolStarted {
				writeSSE(w, "content_block_stop", map[string]any{"type": "content_block_stop", "index": idx})
			}
		} else {
			writeSSE(w, "content_block_stop", map[string]any{"type": "content_block_stop", "index": 0})
		}
	}
	writeSSE(w, "message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": stopReason, "stop_sequence": nil},
		"usage": map[string]any{"output_tokens": 0},
	})
	writeSSE(w, "message_stop", map[string]any{"type": "message_stop"})
}

func writeSSE(w io.Writer, event string, payload any) {
	b, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
}

func decodeSSEDataPayload(body []byte, out any) error {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	var lastData string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		lastData = data
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if lastData == "" {
		return fmt.Errorf("response was not JSON and contained no SSE data payload")
	}
	return json.Unmarshal([]byte(lastData), out)
}
