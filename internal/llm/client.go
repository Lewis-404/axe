package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Lewis-404/axe/internal/config"
)

type Client struct {
	cfg    *config.Config
	http   *http.Client
	tools  []ToolDef
}

func NewClient(cfg *config.Config, tools []ToolDef) *Client {
	return &Client{cfg: cfg, http: &http.Client{}, tools: tools}
}

func (c *Client) Send(system string, messages []Message) (*Response, error) {
	req := Request{
		Model:     c.cfg.Model,
		MaxTokens: c.cfg.MaxTokens,
		System:    system,
		Messages:  messages,
		Tools:     c.tools,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/v1/messages"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if json.Unmarshal(data, &errResp) == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(data))
	}

	var result Response
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &result, nil
}
