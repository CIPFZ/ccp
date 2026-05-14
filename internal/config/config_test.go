package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAPIKeyFromEnvironment(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-test")
	cfg := Config{Providers: map[string]ProviderConfig{
		"deepseek": {Type: "anthropic-compatible", BaseURL: "https://api.deepseek.com/anthropic", APIKey: "${DEEPSEEK_API_KEY}"},
	}}
	if err := cfg.Resolve(); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Providers["deepseek"].ResolvedAPIKey; got != "sk-test" {
		t.Fatalf("ResolvedAPIKey=%q", got)
	}
}

func TestResolveAPIKeyDirect(t *testing.T) {
	cfg := Config{Providers: map[string]ProviderConfig{
		"deepseek": {Type: "anthropic-compatible", BaseURL: "https://api.deepseek.com/anthropic", APIKey: "sk-direct"},
	}}
	if err := cfg.Resolve(); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Providers["deepseek"].ResolvedAPIKey; got != "sk-direct" {
		t.Fatalf("ResolvedAPIKey=%q", got)
	}
}

func TestMissingEnvironmentVariableFails(t *testing.T) {
	os.Unsetenv("MISSING_DEEPSEEK_KEY")
	cfg := Config{Providers: map[string]ProviderConfig{
		"deepseek": {Type: "anthropic-compatible", BaseURL: "https://api.deepseek.com/anthropic", APIKey: "${MISSING_DEEPSEEK_KEY}"},
	}}
	if err := cfg.Resolve(); err == nil {
		t.Fatal("expected missing env error")
	}
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("providers: {}\naliases: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("host=%q", cfg.Server.Host)
	}
	if cfg.Server.Port != 8787 {
		t.Fatalf("port=%d", cfg.Server.Port)
	}
	if cfg.Log.File == "" {
		t.Fatal("expected default log file")
	}
}

func TestLoadUnresolvedDoesNotRequireProviderAPIKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "providers:\n  deepseek:\n    type: anthropic-compatible\n    base_url: https://api.deepseek.com/anthropic\n    api_key: ${MISSING_DEEPSEEK_KEY}\naliases: {}\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadUnresolved(path); err != nil {
		t.Fatal(err)
	}
}

func TestResolveProviderProxyConfig(t *testing.T) {
	cfg := Config{Providers: map[string]ProviderConfig{
		"openai": {
			Type:    "openai-compatible",
			BaseURL: "https://api.openai.com",
			APIKey:  "sk-direct",
			Proxy: ProxyConfig{
				Enabled: true,
				URL:     "http://127.0.0.1:7897",
			},
		},
	}}
	if err := cfg.Resolve(); err != nil {
		t.Fatal(err)
	}
	provider := cfg.Providers["openai"]
	if !provider.Proxy.Enabled || provider.Proxy.URL != "http://127.0.0.1:7897" {
		t.Fatalf("proxy=%+v", provider.Proxy)
	}
}

func TestMaskSecret(t *testing.T) {
	if got := MaskSecret("sk-1234567890"); got != "sk-1...7890" {
		t.Fatalf("masked=%q", got)
	}
	if got := MaskSecret("short"); got != "*****" {
		t.Fatalf("masked short=%q", got)
	}
}
