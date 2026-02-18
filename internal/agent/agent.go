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
	totalIn     int
	totalOut    int
}

func New(client *llm.Client, registry *tools.Registry, systemPrompt string) *Agent {
	return &Agent{
		client:   client,
		registry: registry,
		system:   systemPrompt,
	}
}

func (a *Agent) OnTextDelta(fn func(string))                     { a.onTextDelta = fn }
func (a *Agent) OnBlockDone(fn func())                           { a.onBlockDone = fn }
func (a *Agent) OnTool(fn func(string, string))                  { a.onTool = fn }
func (a *Agent) OnUsage(fn func(int, int, int, int))             { a.onUsage = fn }
func (a *Agent) Messages() []llm.Message                         { return a.messages }
func (a *Agent) SetMessages(msgs []llm.Message)                  { a.messages = msgs }

func (a *Agent) Run(userInput string) error {
	a.messages = append(a.messages, llm.Message{
		Role:    llm.RoleUser,
		Content: []llm.ContentBlock{{Type: "text", Text: userInput}},
	})

	var roundIn, roundOut int

	for {
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

		// parse accumulated JSON input into tool_use blocks
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
	}
}
