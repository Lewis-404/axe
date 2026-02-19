package agent

import (
	"encoding/json"
	"fmt"

	"github.com/Lewis-404/axe/internal/llm"
	"github.com/Lewis-404/axe/internal/tools"
)

type Agent struct {
	client      *llm.Client
	registry    *tools.Registry
	messages    []llm.Message
	system      string
	onTextDelta func(string)
	onBlockDone func()
	onTool      func(string, string)
	onUsage     func(roundIn, roundOut, totalIn, totalOut int)
	onCompact   func(before, after int)
	totalIn     int
	totalOut    int
	maxContext  int // max tokens before auto-compact, 0 = disabled
}

func New(client *llm.Client, registry *tools.Registry, systemPrompt string) *Agent {
	return &Agent{
		client:     client,
		registry:   registry,
		system:     systemPrompt,
		maxContext:  100000, // default 100k
	}
}

func (a *Agent) OnTextDelta(fn func(string))                     { a.onTextDelta = fn }
func (a *Agent) OnBlockDone(fn func())                           { a.onBlockDone = fn }
func (a *Agent) OnTool(fn func(string, string))                  { a.onTool = fn }
func (a *Agent) OnUsage(fn func(int, int, int, int))             { a.onUsage = fn }
func (a *Agent) OnCompact(fn func(int, int))                     { a.onCompact = fn }
func (a *Agent) Messages() []llm.Message                         { return a.messages }
func (a *Agent) SetMessages(msgs []llm.Message)                  { a.messages = msgs }
func (a *Agent) TotalUsage() (int, int)                          { return a.totalIn, a.totalOut }
func (a *Agent) Reset()                                          { a.messages = nil; a.totalIn = 0; a.totalOut = 0 }

// estimateTokens roughly estimates token count (~4 chars per token for mixed CJK/English)
func estimateTokens(msgs []llm.Message) int {
	chars := 0
	for _, m := range msgs {
		for _, b := range m.Content {
			chars += len(b.Text) + len(b.Content) + len(fmt.Sprintf("%v", b.Input))
		}
	}
	return chars / 3 // conservative estimate for mixed content
}

// Compact compresses conversation history into a summary via LLM
func (a *Agent) Compact(hint string) error {
	if len(a.messages) < 4 {
		return nil
	}
	before := estimateTokens(a.messages)

	prompt := "请将以上对话历史压缩为一段简洁的摘要，保留：1) 用户的核心需求 2) 已完成的操作和关键决策 3) 当前进展状态 4) 重要的文件路径和代码上下文。用中文输出。"
	if hint != "" {
		prompt += "\n重点保留: " + hint
	}

	// build summary request with conversation as context
	summaryMsgs := make([]llm.Message, len(a.messages))
	copy(summaryMsgs, a.messages)
	summaryMsgs = append(summaryMsgs, llm.Message{
		Role:    llm.RoleUser,
		Content: []llm.ContentBlock{{Type: "text", Text: prompt}},
	})

	resp, err := a.client.Send(a.system, summaryMsgs)
	if err != nil {
		return fmt.Errorf("compact: %w", err)
	}

	var summary string
	for _, b := range resp.Content {
		if b.Type == "text" {
			summary += b.Text
		}
	}

	a.totalIn += resp.Usage.InputTokens
	a.totalOut += resp.Usage.OutputTokens

	// replace all messages with the compacted summary
	a.messages = []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: "text", Text: "[对话历史摘要]\n" + summary}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: "text", Text: "好的，我已了解之前的对话内容，请继续。"}}},
	}

	after := estimateTokens(a.messages)
	if a.onCompact != nil {
		a.onCompact(before, after)
	}
	return nil
}

func (a *Agent) autoCompact() {
	if a.maxContext <= 0 || len(a.messages) < 6 {
		return
	}
	est := estimateTokens(a.messages)
	if est > a.maxContext*80/100 { // trigger at 80% capacity
		a.Compact("")
	}
}

const maxIterations = 40

func (a *Agent) Run(userInput string) error {
	a.messages = append(a.messages, llm.Message{
		Role:    llm.RoleUser,
		Content: []llm.ContentBlock{{Type: "text", Text: userInput}},
	})

	// check if we need to compact before sending
	a.autoCompact()

	var roundIn, roundOut int
	var consecutiveErrors int

	for iter := 0; iter < maxIterations; iter++ {
		toolInputs := map[int]string{}

		cb := llm.StreamCallbacks{
			OnTextDelta: a.onTextDelta,
			OnBlockStop: func(index int) {
				if a.onBlockDone != nil {
					a.onBlockDone()
				}
			},
			OnInputJSONDelta: func(index int, partial string) {
				toolInputs[index] += partial
			},
		}

		resp, err := a.client.SendStream(a.system, a.messages, cb)
		if err != nil {
			return fmt.Errorf("llm: %w", err)
		}

		roundIn += resp.Usage.InputTokens
		roundOut += resp.Usage.OutputTokens

		for idx, raw := range toolInputs {
			if idx < len(resp.Content) && resp.Content[idx].Type == "tool_use" {
				var parsed any
				if json.Unmarshal([]byte(raw), &parsed) == nil {
					resp.Content[idx].Input = parsed
				}
			}
		}

		a.messages = append(a.messages, llm.Message{
			Role:    llm.RoleAssistant,
			Content: resp.Content,
		})

		var toolResults []llm.ContentBlock
		hasError := false
		for _, block := range resp.Content {
			if block.Type != "tool_use" {
				continue
			}
			if a.onTool != nil {
				a.onTool(block.Name, fmt.Sprintf("%v", block.Input))
			}
			inputBytes, _ := json.Marshal(block.Input)
			result, err := a.registry.Execute(block.Name, inputBytes)
			if err != nil {
				hasError = true
				toolResults = append(toolResults, llm.ContentBlock{
					Type:    "tool_result",
					ToolID:  block.ID,
					Content: fmt.Sprintf("Error: %s", err),
					IsError: true,
				})
			} else {
				if len(result) > 10000 {
					result = result[:10000] + "\n... (truncated)"
				}
				toolResults = append(toolResults, llm.ContentBlock{
					Type:    "tool_result",
					ToolID:  block.ID,
					Content: result,
				})
			}
		}

		if hasError {
			consecutiveErrors++
		} else {
			consecutiveErrors = 0
		}
		if consecutiveErrors >= 3 {
			a.totalIn += roundIn
			a.totalOut += roundOut
			if a.onUsage != nil {
				a.onUsage(roundIn, roundOut, a.totalIn, a.totalOut)
			}
			return fmt.Errorf("3 consecutive tool errors, stopping to avoid loop")
		}

		if len(toolResults) == 0 {
			a.totalIn += roundIn
			a.totalOut += roundOut
			if a.onUsage != nil {
				a.onUsage(roundIn, roundOut, a.totalIn, a.totalOut)
			}
			return nil
		}

		a.messages = append(a.messages, llm.Message{
			Role:    llm.RoleUser,
			Content: toolResults,
		})

		// check context size mid-loop
		a.autoCompact()
	}

	a.totalIn += roundIn
	a.totalOut += roundOut
	if a.onUsage != nil {
		a.onUsage(roundIn, roundOut, a.totalIn, a.totalOut)
	}
	return fmt.Errorf("reached max iterations (%d), task may be incomplete", maxIterations)
}
