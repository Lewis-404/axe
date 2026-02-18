package llm

import (
	"bufio"
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

func (c *Client) SendStream(system string, messages []Message, cb StreamCallbacks) (*Response, error) {
	req := struct {
		Request
		Stream bool `json:"stream"`
	}{
		Request: Request{
			Model:     c.cfg.Model,
			MaxTokens: c.cfg.MaxTokens,
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
				// grow content slice to fit index
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
