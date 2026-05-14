package model

import "testing"

func TestResolveAliasDirect(t *testing.T) {
	r, err := Resolve("sonnet", map[string]string{"sonnet": "deepseek:deepseek-v4-pro"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Provider != "deepseek" || r.Model != "deepseek-v4-pro" || r.Alias != "sonnet" {
		t.Fatalf("unexpected route: %+v", r)
	}
}

func TestResolveClaudeOpusModelUsesOpusAlias(t *testing.T) {
	r, err := Resolve("claude-opus-4-6-20260101", map[string]string{"opus": "deepseek:deepseek-v4-pro"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Alias != "opus" {
		t.Fatalf("alias=%q", r.Alias)
	}
}

func TestInvalidAliasTargetFails(t *testing.T) {
	_, err := Resolve("sonnet", map[string]string{"sonnet": "bad-target"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProviderModelRoutesDirectly(t *testing.T) {
	r, err := Resolve("deepseek:deepseek-v4-flash", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Provider != "deepseek" || r.Model != "deepseek-v4-flash" {
		t.Fatalf("unexpected route: %+v", r)
	}
}
