package agent

import (
	"encoding/json"
	"fmt"

	"github.com/Lewis-404/axe/internal/llm"
	"github.com/Lewis-404/axe/internal/tools"
)

type Agent struct {
	client   *llm.Client
	registry *tools.Registry
	messages []llm.Message
	system   string
	onText   func(string)
	onTool   func(string, string)
}

func New(client *llm.Client, registry *tools.Registry, systemPrompt string) *Agent {
	return &Agent{
		client:   client,
		registry: registry,
		system:   systemPrompt,
	}
}

func (a *Agent) OnText(fn func(string))       { a.onText = fn }
func (a *Agent) OnTool(fn func(string, string)) { a.onTool = fn }

func (a *Agent) Run(userInput string) error {
	a.messages = append(a.messages, llm.Message{
		Role: llm.RoleUser,
		Content: []llm.ContentBlock{{Type: "text", Text: userInput}},
	})

	for {
		resp, err := a.client.Send(a.system, a.messages)
		if err != nil {
			return fmt.Errorf("llm: %w", err)
		}

		// collect assistant response
		a.messages = append(a.messages, llm.Message{
			Role:    llm.RoleAssistant,
			Content: resp.Content,
		})

		// process response blocks
		var toolResults []llm.ContentBlock
		for _, block := range resp.Content {
			switch block.Type {
			case "text":
				if a.onText != nil {
					a.onText(block.Text)
				}
			case "tool_use":
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
		}

		// if no tool calls, we're done
		if len(toolResults) == 0 {
			return nil
		}

		// send tool results back
		a.messages = append(a.messages, llm.Message{
			Role:    llm.RoleUser,
			Content: toolResults,
		})
	}
}
