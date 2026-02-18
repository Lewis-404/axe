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

// OpenAI wire types

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    any           `json:"content,omitempty"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type oaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaiTool struct {
	Type     string      `json:"type"`
	Function oaiFunction `json:"function"`
}

type oaiFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type oaiRequest struct {
	Model     string       `json:"model"`
	Messages  []oaiMessage `json:"messages"`
	Tools     []oaiTool    `json:"tools,omitempty"`
	MaxTokens int          `json:"max_tokens,omitempty"`
	Stream    bool         `json:"stream,omitempty"`
}

type oaiChoice struct {
	Message      oaiRespMessage `json:"message"`
	FinishReason string         `json:"finish_reason"`
}

type oaiRespMessage struct {
	Role      string        `json:"role"`
	Content   string        `json:"content"`
	ToolCalls []oaiToolCall `json:"tool_calls,omitempty"`
}

type oaiResponse struct {
	ID      string      `json:"id"`
	Choices []oaiChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

type oaiStreamDelta struct {
	Role      string             `json:"role,omitempty"`
	Content   string             `json:"content,omitempty"`
	ToolCalls []oaiStreamToolDelta `json:"tool_calls,omitempty"`
}

type oaiStreamToolDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
}

type oaiStreamChunk struct {
	ID      string `json:"id"`
	Choices []struct {
		Delta        oaiStreamDelta `json:"delta"`
		FinishReason *string        `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage,omitempty"`
}

// OpenAI client

type OpenAIClient struct {
	cfg   *config.Config
	http  *http.Client
	tools []ToolDef
}

func NewOpenAIClient(cfg *config.Config, tools []ToolDef) *OpenAIClient {
	return &OpenAIClient{cfg: cfg, http: &http.Client{}, tools: tools}
}

func (c *OpenAIClient) convertTools() []oaiTool {
	if len(c.tools) == 0 {
		return nil
	}
	out := make([]oaiTool, len(c.tools))
	for i, t := range c.tools {
		out[i] = oaiTool{
			Type: "function",
			Function: oaiFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		}
	}
	return out
}

func (c *OpenAIClient) convertMessages(system string, messages []Message) []oaiMessage {
	var out []oaiMessage
	if system != "" {
		out = append(out, oaiMessage{Role: "system", Content: system})
	}
	for _, m := range messages {
		// check if this is a tool_result message
		if len(m.Content) > 0 && m.Content[0].Type == "tool_result" {
			for _, b := range m.Content {
				out = append(out, oaiMessage{
					Role:       "tool",
					Content:    b.Content,
					ToolCallID: b.ToolID,
				})
			}
			continue
		}
		msg := oaiMessage{Role: string(m.Role)}
		// check for tool_use blocks (assistant with tool calls)
		var textParts []string
		var toolCalls []oaiToolCall
		for _, b := range m.Content {
			switch b.Type {
			case "text":
				if b.Text != "" {
					textParts = append(textParts, b.Text)
				}
			case "tool_use":
				args, _ := json.Marshal(b.Input)
				toolCalls = append(toolCalls, oaiToolCall{
					ID:   b.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: b.Name, Arguments: string(args)},
				})
			}
		}
		if len(textParts) > 0 {
			msg.Content = strings.Join(textParts, "\n")
		}
		if len(toolCalls) > 0 {
			msg.ToolCalls = toolCalls
		}
		out = append(out, msg)
	}
	return out
}

func (c *OpenAIClient) doRequest(body []byte) (*http.Response, error) {
	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	return c.http.Do(req)
}

func (c *OpenAIClient) parseResponse(oaiResp *oaiResponse) *Response {
	resp := &Response{
		ID:   oaiResp.ID,
		Role: RoleAssistant,
		Usage: Usage{
			InputTokens:  oaiResp.Usage.PromptTokens,
			OutputTokens: oaiResp.Usage.CompletionTokens,
		},
	}
	if len(oaiResp.Choices) == 0 {
		return resp
	}
	ch := oaiResp.Choices[0]
	resp.StopReason = convertStopReason(ch.FinishReason)
	if ch.Message.Content != "" {
		resp.Content = append(resp.Content, ContentBlock{Type: "text", Text: ch.Message.Content})
	}
	for _, tc := range ch.Message.ToolCalls {
		var input any
		json.Unmarshal([]byte(tc.Function.Arguments), &input)
		resp.Content = append(resp.Content, ContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}
	return resp
}

func convertStopReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	default:
		return reason
	}
}

func (c *OpenAIClient) Send(system string, messages []Message) (*Response, error) {
	reqBody := oaiRequest{
		Model:     c.cfg.Model,
		Messages:  c.convertMessages(system, messages),
		Tools:     c.convertTools(),
		MaxTokens: c.cfg.MaxTokens,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.doRequest(body)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(data))
	}

	var oaiResp oaiResponse
	if err := json.Unmarshal(data, &oaiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return c.parseResponse(&oaiResp), nil
}

func (c *OpenAIClient) SendStream(system string, messages []Message, cb StreamCallbacks) (*Response, error) {
	reqBody := oaiRequest{
		Model:     c.cfg.Model,
		Messages:  c.convertMessages(system, messages),
		Tools:     c.convertTools(),
		MaxTokens: c.cfg.MaxTokens,
		Stream:    true,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.doRequest(body)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(data))
	}

	result := &Response{Role: RoleAssistant}
	// track tool call accumulation: index -> {id, name, arguments}
	type toolAcc struct {
		id   string
		name string
		args string
	}
	toolAccs := map[int]*toolAcc{}
	hasText := false
	textBlockIdx := -1

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}

		var chunk oaiStreamChunk
		if json.Unmarshal([]byte(payload), &chunk) != nil {
			continue
		}

		if result.ID == "" {
			result.ID = chunk.ID
		}
		if chunk.Usage != nil {
			result.Usage.InputTokens = chunk.Usage.PromptTokens
			result.Usage.OutputTokens = chunk.Usage.CompletionTokens
		}

		for _, ch := range chunk.Choices {
			if ch.FinishReason != nil {
				result.StopReason = convertStopReason(*ch.FinishReason)
			}

			// text content delta
			if ch.Delta.Content != "" {
				if !hasText {
					hasText = true
					textBlockIdx = len(result.Content)
					result.Content = append(result.Content, ContentBlock{Type: "text"})
					if cb.OnBlockStart != nil {
						cb.OnBlockStart(textBlockIdx, result.Content[textBlockIdx])
					}
				}
				result.Content[textBlockIdx].Text += ch.Delta.Content
				if cb.OnTextDelta != nil {
					cb.OnTextDelta(ch.Delta.Content)
				}
			}

			// tool call deltas
			for _, tc := range ch.Delta.ToolCalls {
				acc, exists := toolAccs[tc.Index]
				if !exists {
					acc = &toolAcc{}
					toolAccs[tc.Index] = acc
				}
				if tc.ID != "" {
					acc.id = tc.ID
				}
				if tc.Function.Name != "" {
					acc.name += tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					acc.args += tc.Function.Arguments
					if cb.OnInputJSONDelta != nil {
						// use a content index offset: text block (if any) + tool index
						contentIdx := tc.Index
						if hasText {
							contentIdx = textBlockIdx + 1 + tc.Index
						}
						cb.OnInputJSONDelta(contentIdx, tc.Function.Arguments)
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read stream: %w", err)
	}

	// finalize text block
	if hasText && cb.OnBlockStop != nil {
		cb.OnBlockStop(textBlockIdx)
	}

	// build tool_use content blocks from accumulated deltas
	for i := 0; i < len(toolAccs); i++ {
		acc := toolAccs[i]
		var input any
		json.Unmarshal([]byte(acc.args), &input)
		block := ContentBlock{
			Type:  "tool_use",
			ID:    acc.id,
			Name:  acc.name,
			Input: input,
		}
		idx := len(result.Content)
		result.Content = append(result.Content, block)
		if cb.OnBlockStart != nil {
			cb.OnBlockStart(idx, block)
		}
		if cb.OnBlockStop != nil {
			cb.OnBlockStop(idx)
		}
	}

	if cb.OnMessageDone != nil {
		cb.OnMessageDone(result)
	}

	return result, nil
}
