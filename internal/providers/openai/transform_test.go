package openaiprovider

import (
	"encoding/json"
	"strings"
	"testing"

	"ccp/internal/anthropic"
)

func TestBuildChatRequestConvertsSystemAndUserText(t *testing.T) {
	req := anthropic.MessageRequest{
		Model:     "sonnet",
		System:    "be concise",
		MaxTokens: 100,
		Messages:  []anthropic.Message{{Role: "user", Content: "hello"}},
	}
	chat, err := buildChatRequest("deepseek-v4-pro", req)
	if err != nil {
		t.Fatal(err)
	}
	if chat.Model != "deepseek-v4-pro" {
		t.Fatalf("model=%q", chat.Model)
	}
	if len(chat.Messages) != 2 {
		t.Fatalf("messages=%d", len(chat.Messages))
	}
	if chat.Messages[0].Role != "system" || chat.Messages[0].Content != "be concise" {
		t.Fatalf("system message=%+v", chat.Messages[0])
	}
	if chat.Messages[1].Role != "user" || chat.Messages[1].Content != "hello" {
		t.Fatalf("user message=%+v", chat.Messages[1])
	}
}

func TestConvertOpenAIStreamToolCallDeltas(t *testing.T) {
	input := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"Read","arguments":"{\"file_"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"path\":\"a.txt\"}"}}]}}]}`,
		`data: [DONE]`,
		``,
	}, "\n")
	var out strings.Builder
	convertOpenAIStream(strings.NewReader(input), &out, "deepseek-v4-pro")
	got := out.String()
	for _, want := range []string{
		"event: content_block_start",
		`"type":"tool_use"`,
		`"id":"call_1"`,
		`"name":"Read"`,
		"event: content_block_delta",
		`"type":"input_json_delta"`,
		`"{\"file_`,
		`path\":\"a.txt\"}"`,
		"event: content_block_stop",
		`"stop_reason":"tool_use"`,
		"event: message_stop",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in stream:\n%s", want, got)
		}
	}
}

func TestBuildChatRequestConvertsTools(t *testing.T) {
	req := anthropic.MessageRequest{
		Messages: []anthropic.Message{{Role: "user", Content: "read file"}},
		Tools: []anthropic.Tool{{
			Name:        "Read",
			Description: "Read a file",
			InputSchema: map[string]any{
				"type": "object",
			},
		}},
	}
	chat, err := buildChatRequest("deepseek-v4-pro", req)
	if err != nil {
		t.Fatal(err)
	}
	if len(chat.Tools) != 1 {
		t.Fatalf("tools=%d", len(chat.Tools))
	}
	if chat.Tools[0].Type != "function" || chat.Tools[0].Function.Name != "Read" {
		t.Fatalf("tool=%+v", chat.Tools[0])
	}
}

func TestBuildChatRequestConvertsAssistantToolUseAndUserToolResult(t *testing.T) {
	req := anthropic.MessageRequest{
		Messages: []anthropic.Message{
			{
				Role: "assistant",
				Content: []any{map[string]any{
					"type":  "tool_use",
					"id":    "call_1",
					"name":  "Read",
					"input": map[string]any{"file_path": "a.txt"},
				}},
			},
			{
				Role: "user",
				Content: []any{map[string]any{
					"type":        "tool_result",
					"tool_use_id": "call_1",
					"content":     "file text",
				}},
			},
		},
	}
	chat, err := buildChatRequest("deepseek-v4-pro", req)
	if err != nil {
		t.Fatal(err)
	}
	if len(chat.Messages) != 2 {
		t.Fatalf("messages=%+v", chat.Messages)
	}
	assistant := chat.Messages[0]
	if assistant.Role != "assistant" || len(assistant.ToolCalls) != 1 {
		t.Fatalf("assistant=%+v", assistant)
	}
	if assistant.ToolCalls[0].ID != "call_1" || assistant.ToolCalls[0].Function.Name != "Read" {
		t.Fatalf("tool call=%+v", assistant.ToolCalls[0])
	}
	tool := chat.Messages[1]
	if tool.Role != "tool" || tool.ToolCallID != "call_1" || tool.Content != "file text" {
		t.Fatalf("tool result=%+v", tool)
	}
}

func TestConvertChatResponseText(t *testing.T) {
	resp := chatResponse{
		ID: "chatcmpl_1",
		Choices: []chatChoice{{
			Message: chatMessage{Role: "assistant", Content: "hello"},
		}},
		Usage: &chatUsage{PromptTokens: 10, CompletionTokens: 5},
	}
	out, err := convertChatResponse("deepseek-v4-pro", resp)
	if err != nil {
		t.Fatal(err)
	}
	if out.Model != "deepseek-v4-pro" || out.Content[0].Text != "hello" {
		t.Fatalf("out=%+v", out)
	}
	if out.Usage.InputTokens != 10 || out.Usage.OutputTokens != 5 {
		t.Fatalf("usage=%+v", out.Usage)
	}
}

func TestConvertChatResponseToolCall(t *testing.T) {
	resp := chatResponse{
		ID: "chatcmpl_1",
		Choices: []chatChoice{{
			Message: chatMessage{
				Role: "assistant",
				ToolCalls: []chatToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: chatToolCallFunction{
						Name:      "Read",
						Arguments: `{"file_path":"a.txt"}`,
					},
				}},
			},
		}},
	}
	out, err := convertChatResponse("deepseek-v4-pro", resp)
	if err != nil {
		t.Fatal(err)
	}
	if out.StopReason != "tool_use" {
		t.Fatalf("stop_reason=%q", out.StopReason)
	}
	block := out.Content[0]
	if block.Type != "tool_use" || block.ID != "call_1" || block.Name != "Read" {
		t.Fatalf("block=%+v", block)
	}
	b, _ := json.Marshal(block.Input)
	if string(b) != `{"file_path":"a.txt"}` {
		t.Fatalf("input=%s", string(b))
	}
}
