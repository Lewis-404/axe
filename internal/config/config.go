package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ModelConfig struct {
	Provider  string `yaml:"provider"`
	APIKey    string `yaml:"api_key"`
	BaseURL   string `yaml:"base_url"`
	Model     string `yaml:"model"`
	MaxTokens int    `yaml:"max_tokens"`
}

func (m *ModelConfig) IsOpenAI() bool {
	return m.Provider == "openai"
}

type Config struct {
	Models []ModelConfig `yaml:"models"`
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".axe")
}

func configPath() string {
	return filepath.Join(configDir(), "config.yaml")
}

func Load() (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config not found, run 'axe init' first")
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// env overrides: apply to matching provider entries
	for i := range cfg.Models {
		m := &cfg.Models[i]
		if m.IsOpenAI() {
			if key := os.Getenv("OPENAI_API_KEY"); key != "" && m.APIKey == "" {
				m.APIKey = key
			}
			if url := os.Getenv("OPENAI_BASE_URL"); url != "" && m.BaseURL == "" {
				m.BaseURL = url
			}
		} else {
			if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" && m.APIKey == "" {
				m.APIKey = key
			}
			if url := os.Getenv("ANTHROPIC_BASE_URL"); url != "" && m.BaseURL == "" {
				m.BaseURL = url
			}
		}
		// defaults
		if m.MaxTokens == 0 {
			m.MaxTokens = 8192
		}
		if m.Provider == "" {
			m.Provider = "anthropic"
		}
	}

	// validate: at least one model with api_key
	valid := 0
	for _, m := range cfg.Models {
		if m.APIKey != "" && m.Model != "" {
			valid++
		}
	}
	if valid == 0 {
		return nil, fmt.Errorf("no valid model config found (need at least api_key + model)")
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

	template := `# Axe 配置文件
# 至少配置一个模型，支持多个模型自动 fallback
models:
  - provider: anthropic          # anthropic 或 openai
    api_key: "your-api-key"
    base_url: "https://api.anthropic.com"
    model: "claude-sonnet-4-20250514"
    max_tokens: 8192

  # 备用模型（可选，第一个失败时自动切换）
  # - provider: openai
  #   api_key: "sk-xxx"
  #   base_url: "https://api.openai.com"
  #   model: "gpt-4o"
  #   max_tokens: 8192
`
	return os.WriteFile(path, []byte(template), 0600)
}
