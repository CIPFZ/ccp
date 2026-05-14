package openaiprovider

import (
	"encoding/json"
	"fmt"

	"ccp/internal/anthropic"
)

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Tools       []chatTool    `json:"tools,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type chatTool struct {
	Type     string           `json:"type"`
	Function chatToolFunction `json:"function"`
}

type chatToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type chatToolCall struct {
	Index    int                  `json:"index,omitempty"`
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function chatToolCallFunction `json:"function"`
}

type chatToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatResponse struct {
	ID      string       `json:"id"`
	Choices []chatChoice `json:"choices"`
	Usage   *chatUsage   `json:"usage,omitempty"`
}

type chatChoice struct {
	Message      chatMessage `json:"message"`
	Delta        chatMessage `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func buildChatRequest(model string, req anthropic.MessageRequest) (chatRequest, error) {
	out := chatRequest{
		Model:       model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
	}
	if req.System != nil {
		text, err := contentToText(req.System)
		if err != nil {
			return out, err
		}
		if text != "" {
			out.Messages = append(out.Messages, chatMessage{Role: "system", Content: text})
		}
	}
	for _, msg := range req.Messages {
		converted, err := convertAnthropicMessage(msg)
		if err != nil {
			return out, err
		}
		out.Messages = append(out.Messages, converted...)
	}
	for _, tool := range req.Tools {
		out.Tools = append(out.Tools, chatTool{
			Type: "function",
			Function: chatToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}
	return out, nil
}

func convertAnthropicMessage(msg anthropic.Message) ([]chatMessage, error) {
	if blocks, ok := msg.Content.([]any); ok {
		var out []chatMessage
		var text string
		var toolCalls []chatToolCall
		for _, block := range blocks {
			m, ok := block.(map[string]any)
			if !ok {
				continue
			}
			switch m["type"] {
			case "text":
				text += fmt.Sprint(m["text"])
			case "tool_result":
				out = append(out, chatMessage{
					Role:       "tool",
					ToolCallID: fmt.Sprint(m["tool_use_id"]),
					Content:    fmt.Sprint(m["content"]),
				})
			case "tool_use":
				args, err := json.Marshal(m["input"])
				if err != nil {
					return nil, err
				}
				toolCalls = append(toolCalls, chatToolCall{
					ID:   fmt.Sprint(m["id"]),
					Type: "function",
					Function: chatToolCallFunction{
						Name:      fmt.Sprint(m["name"]),
						Arguments: string(args),
					},
				})
			}
		}
		if len(toolCalls) > 0 {
			out = append([]chatMessage{{Role: "assistant", Content: text, ToolCalls: toolCalls}}, out...)
			return out, nil
		}
		if text != "" {
			out = append([]chatMessage{{Role: msg.Role, Content: text}}, out...)
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	if msg.Role == "assistant" {
		return []chatMessage{{Role: "assistant", Content: fmt.Sprint(msg.Content)}}, nil
	}
	text, err := contentToText(msg.Content)
	if err != nil {
		return nil, err
	}
	return []chatMessage{{Role: msg.Role, Content: text}}, nil
}

func contentToText(content any) (string, error) {
	switch v := content.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case []any:
		var text string
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if m["type"] == "text" {
				text += fmt.Sprint(m["text"])
			}
		}
		return text, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

func convertChatResponse(model string, resp chatResponse) (*anthropic.MessageResponse, error) {
	out := &anthropic.MessageResponse{
		ID:         resp.ID,
		Type:       "message",
		Role:       "assistant",
		Model:      model,
		StopReason: "end_turn",
	}
	if out.ID == "" {
		out.ID = "msg_ccp"
	}
	if len(resp.Choices) > 0 {
		msg := resp.Choices[0].Message
		if len(msg.ToolCalls) > 0 {
			out.StopReason = "tool_use"
			for _, call := range msg.ToolCalls {
				var input any = map[string]any{}
				if call.Function.Arguments != "" {
					if err := json.Unmarshal([]byte(call.Function.Arguments), &input); err != nil {
						input = map[string]any{"arguments": call.Function.Arguments}
					}
				}
				out.Content = append(out.Content, anthropic.ContentBlock{
					Type:  "tool_use",
					ID:    call.ID,
					Name:  call.Function.Name,
					Input: input,
				})
			}
		} else {
			out.Content = append(out.Content, anthropic.ContentBlock{Type: "text", Text: msg.Content})
		}
	}
	if resp.Usage != nil {
		out.Usage = &anthropic.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		}
	}
	return out, nil
}
