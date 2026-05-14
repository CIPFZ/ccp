package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendWritesJSONLAndCreatesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "usage.jsonl")
	err := Append(path, Record{
		RequestID:    "req_1",
		Timestamp:    "2026-05-13T12:00:00+08:00",
		Provider:     "deepseek",
		Model:        "deepseek-v4-pro",
		Alias:        "sonnet",
		Status:       200,
		DurationMS:   10,
		InputTokens:  1,
		OutputTokens: 2,
		TotalTokens:  3,
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := string(b)
	if !strings.Contains(line, `"request_id":"req_1"`) || !strings.HasSuffix(line, "\n") {
		t.Fatalf("line=%q", line)
	}
}
