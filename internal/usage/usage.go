package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Record struct {
	RequestID    string `json:"request_id"`
	Timestamp    string `json:"ts"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Alias        string `json:"alias"`
	Stream       bool   `json:"stream"`
	Status       int    `json:"status"`
	DurationMS   int64  `json:"duration_ms"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
	TotalTokens  int    `json:"total_tokens,omitempty"`
	Estimated    bool   `json:"estimated"`
	Error        string `json:"error,omitempty"`
}

func Append(path string, record Record) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}
