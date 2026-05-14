package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig              `yaml:"server"`
	Log       LogConfig                 `yaml:"log"`
	Usage     UsageConfig               `yaml:"usage"`
	Aliases   map[string]string         `yaml:"aliases"`
	Providers map[string]ProviderConfig `yaml:"providers"`
}

type ServerConfig struct {
	Host                  string `yaml:"host"`
	Port                  int    `yaml:"port"`
	MaxConcurrentRequests int    `yaml:"max_concurrent_requests"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	File   string `yaml:"file"`
	Prompt bool   `yaml:"prompt"`
}

type UsageConfig struct {
	Enabled bool   `yaml:"enabled"`
	File    string `yaml:"file"`
}

type ProviderConfig struct {
	Type                  string            `yaml:"type"`
	BaseURL               string            `yaml:"base_url"`
	APIKey                string            `yaml:"api_key"`
	ResolvedAPIKey        string            `yaml:"-"`
	Proxy                 ProxyConfig       `yaml:"proxy"`
	MaxConcurrentRequests int               `yaml:"max_concurrent_requests"`
	Headers               map[string]string `yaml:"headers"`
}

type ProxyConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
}

var envRefPattern = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)

func DefaultPath() string {
	return filepath.Join(homeDir(), ".ccp", "config.yaml")
}

func Load(path string) (*Config, error) {
	cfg, err := LoadUnresolved(path)
	if err != nil {
		return nil, err
	}
	if err := cfg.Resolve(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func LoadUnresolved(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath()
	}
	b, err := os.ReadFile(ExpandPath(path))
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	cfg.ApplyDefaults()
	return &cfg, nil
}

func (c *Config) ApplyDefaults() {
	if c.Server.Host == "" {
		c.Server.Host = "127.0.0.1"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8787
	}
	if c.Server.MaxConcurrentRequests == 0 {
		c.Server.MaxConcurrentRequests = 64
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.File == "" {
		c.Log.File = filepath.Join(homeDir(), ".ccp", "logs", "ccp.log")
	}
	c.Log.File = ExpandPath(c.Log.File)
	if c.Usage.File == "" {
		c.Usage.File = filepath.Join(homeDir(), ".ccp", "logs", "usage.jsonl")
	}
	c.Usage.File = ExpandPath(c.Usage.File)
	if c.Aliases == nil {
		c.Aliases = map[string]string{}
	}
	if c.Providers == nil {
		c.Providers = map[string]ProviderConfig{}
	}
}

func (c *Config) Resolve() error {
	c.ApplyDefaults()
	for name, provider := range c.Providers {
		provider.Type = strings.TrimSpace(provider.Type)
		provider.BaseURL = strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
		provider.APIKey = strings.TrimSpace(provider.APIKey)
		provider.Proxy.URL = strings.TrimSpace(provider.Proxy.URL)
		if provider.Type == "" {
			return fmt.Errorf("provider %q missing type", name)
		}
		if provider.BaseURL == "" {
			return fmt.Errorf("provider %q missing base_url", name)
		}
		if provider.Proxy.Enabled && provider.Proxy.URL == "" {
			return fmt.Errorf("provider %q proxy.url is required when proxy.enabled is true", name)
		}
		if provider.MaxConcurrentRequests == 0 {
			provider.MaxConcurrentRequests = 16
		}
		if provider.MaxConcurrentRequests < 0 {
			return fmt.Errorf("provider %q max_concurrent_requests must be >= 0", name)
		}
		key, err := resolveAPIKey(provider.APIKey)
		if err != nil {
			return fmt.Errorf("provider %q api_key: %w", name, err)
		}
		provider.ResolvedAPIKey = key
		c.Providers[name] = provider
	}
	return nil
}

func ExpandPath(path string) string {
	if path == "" {
		return ""
	}
	if path == "~" {
		return homeDir()
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		return filepath.Join(homeDir(), path[2:])
	}
	return path
}

func MaskSecret(secret string) string {
	if len(secret) <= 8 {
		return strings.Repeat("*", len(secret))
	}
	return secret[:4] + "..." + secret[len(secret)-4:]
}

func resolveAPIKey(value string) (string, error) {
	if value == "" {
		return "", errors.New("missing value")
	}
	if match := envRefPattern.FindStringSubmatch(value); match != nil {
		key := os.Getenv(match[1])
		if key == "" {
			return "", fmt.Errorf("environment variable %s is empty or unset", match[1])
		}
		return key, nil
	}
	return value, nil
}

func homeDir() string {
	if runtime.GOOS == "windows" {
		if home := os.Getenv("USERPROFILE"); home != "" {
			return home
		}
	}
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if dir, err := os.UserHomeDir(); err == nil {
		return dir
	}
	return "."
}
