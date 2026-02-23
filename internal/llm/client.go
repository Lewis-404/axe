package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Lewis-404/axe/internal/config"
)

// Provider is the interface both Anthropic and OpenAI backends implement.
type Provider interface {
	Send(system string, messages []Message) (*Response, error)
	SendStream(system string, messages []Message, cb StreamCallbacks) (*Response, error)
	ModelName() string
}

// Client wraps multiple Providers with fallback support.
type Client struct {
	providers []Provider
	activeIdx int
}

func NewClient(models []config.ModelConfig, tools []ToolDef) *Client {
	var providers []Provider
	for i := range models {
		m := &models[i]
		if m.APIKey == "" || m.Model == "" {
			continue
		}
		if m.IsOpenAI() {
			providers = append(providers, NewOpenAIClient(m, tools))
		} else {
			providers = append(providers, NewAnthropicClient(m, tools))
		}
	}
	return &Client{providers: providers}
}

func (c *Client) Send(system string, messages []Message) (*Response, error) {
	var lastErr error
	for i := 0; i < len(c.providers); i++ {
		idx := (c.activeIdx + i) % len(c.providers)
		resp, err := c.providers[idx].Send(system, messages)
		if err == nil {
			c.activeIdx = idx
			return resp, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (c *Client) SendStream(system string, messages []Message, cb StreamCallbacks) (*Response, error) {
	var lastErr error
	for i := 0; i < len(c.providers); i++ {
		idx := (c.activeIdx + i) % len(c.providers)
		resp, err := c.providers[idx].SendStream(system, messages, cb)
		if err == nil {
			c.activeIdx = idx
			return resp, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (c *Client) ModelName() string {
	if len(c.providers) == 0 {
		return "none"
	}
	return c.providers[c.activeIdx].ModelName()
}

func (c *Client) SwitchModel(name string) bool {
	for i, p := range c.providers {
		if p.ModelName() == name {
			c.activeIdx = i
			return true
		}
	}
	return false
}

func (c *Client) ListModels() []string {
	var names []string
	for _, p := range c.providers {
		names = append(names, p.ModelName())
	}
	return names
}

// AnthropicClient implements Provider for the Anthropic API.
type AnthropicClient struct {
	model *config.ModelConfig
	http  *http.Client
	tools []ToolDef
}

func NewAnthropicClient(m *config.ModelConfig, tools []ToolDef) *AnthropicClient {
	return &AnthropicClient{model: m, http: &http.Client{Timeout: 5 * time.Minute}, tools: tools}
}

func (c *AnthropicClient) ModelName() string { return c.model.Model }

func (c *AnthropicClient) Send(system string, messages []Message) (*Response, error) {
	req := Request{
		Model:     c.model.Model,
		MaxTokens: c.model.MaxTokens,
		System:    system,
		Messages:  messages,
		Tools:     c.tools,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(c.model.BaseURL, "/") + "/v1/messages"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.model.APIKey)
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

func (c *AnthropicClient) SendStream(system string, messages []Message, cb StreamCallbacks) (*Response, error) {
	req := struct {
		Request
		Stream bool `json:"stream"`
	}{
		Request: Request{
			Model:     c.model.Model,
			MaxTokens: c.model.MaxTokens,
			System:    system,
			Messages:  messages,
			Tools:     c.tools,
		},
		Stream: true,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(c.model.BaseURL, "/") + "/v1/messages"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.model.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	// retry on rate limit (429) or server error (529/500)
	if resp.StatusCode == 429 || resp.StatusCode == 529 || resp.StatusCode == 500 {
		resp.Body.Close()
		for retry := 0; retry < 3; retry++ {
			wait := time.Duration(2<<retry) * time.Second
			fmt.Fprintf(os.Stderr, "⏳ API %d, retrying in %s...\n", resp.StatusCode, wait)
			time.Sleep(wait)
			httpReq2, _ := http.NewRequest("POST", url, bytes.NewReader(body))
			httpReq2.Header.Set("Content-Type", "application/json")
			httpReq2.Header.Set("x-api-key", c.model.APIKey)
			httpReq2.Header.Set("anthropic-version", "2023-06-01")
			resp, err = c.http.Do(httpReq2)
			if err != nil {
				return nil, fmt.Errorf("send request: %w", err)
			}
			if resp.StatusCode == 200 {
				break
			}
			resp.Body.Close()
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("API error (%d) after retries", resp.StatusCode)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		if json.Unmarshal(data, &errResp) == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(data))
	}

	var result Response
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := []byte(strings.TrimPrefix(line, "data: "))

		var ev StreamEvent
		if json.Unmarshal(data, &ev) != nil {
			continue
		}

		switch ev.Type {
		case "message_start":
			var e MessageStartEvent
			if json.Unmarshal(data, &e) == nil {
				result = e.Message
				result.Content = nil
			}
		case "content_block_start":
			var e ContentBlockStartEvent
			if json.Unmarshal(data, &e) == nil {
				for len(result.Content) <= e.Index {
					result.Content = append(result.Content, ContentBlock{})
				}
				result.Content[e.Index] = e.ContentBlock
				if cb.OnBlockStart != nil {
					cb.OnBlockStart(e.Index, e.ContentBlock)
				}
			}
		case "content_block_delta":
			var e ContentBlockDeltaEvent
			if json.Unmarshal(data, &e) == nil {
				switch e.Delta.Type {
				case "text_delta":
					if e.Index < len(result.Content) {
						result.Content[e.Index].Text += e.Delta.Text
					}
					if cb.OnTextDelta != nil {
						cb.OnTextDelta(e.Delta.Text)
					}
				case "input_json_delta":
					if cb.OnInputJSONDelta != nil {
						cb.OnInputJSONDelta(e.Index, e.Delta.PartialJSON)
					}
				}
			}
		case "content_block_stop":
			if cb.OnBlockStop != nil {
				var e struct {
					Index int `json:"index"`
				}
				if json.Unmarshal(data, &e) == nil {
					cb.OnBlockStop(e.Index)
				}
			}
		case "message_delta":
			var e MessageDeltaEvent
			if json.Unmarshal(data, &e) == nil {
				result.StopReason = e.Delta.StopReason
			}
		case "message_stop":
			if cb.OnMessageDone != nil {
				cb.OnMessageDone(&result)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read stream: %w", err)
	}

	return &result, nil
}
