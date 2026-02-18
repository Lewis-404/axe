package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Provider  string `yaml:"provider"`
	APIKey    string `yaml:"api_key"`
	BaseURL   string `yaml:"base_url"`
	Model     string `yaml:"model"`
	MaxTokens int    `yaml:"max_tokens"`
}

func (c *Config) IsOpenAI() bool {
	return c.Provider == "openai"
}

func (c *Config) SetModel(model string) {
	c.Model = model
}

func DefaultConfig() *Config {
	return &Config{
		BaseURL:   "https://api.anthropic.com",
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 8192,
	}
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".axe")
}

func configPath() string {
	return filepath.Join(configDir(), "config.yaml")
}

func Load() (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(configPath())
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	// env overrides
	if cfg.IsOpenAI() {
		if key := os.Getenv("OPENAI_API_KEY"); key != "" {
			cfg.APIKey = key
		}
		if url := os.Getenv("OPENAI_BASE_URL"); url != "" {
			cfg.BaseURL = url
		}
	} else {
		if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
			cfg.APIKey = key
		}
		if url := os.Getenv("ANTHROPIC_BASE_URL"); url != "" {
			cfg.BaseURL = url
		}
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key not set in config or environment")
	}
	return cfg, nil
}

func Init() error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path := configPath()
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists: %s", path)
	}

	cfg := DefaultConfig()
	cfg.APIKey = "your-api-key-here"

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
