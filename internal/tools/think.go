package tools

import "encoding/json"

// Think is a no-op tool that lets the LLM plan before acting.
type Think struct{}

func (t *Think) Name() string        { return "think" }
func (t *Think) Description() string { return "Plan your approach before making changes. Use this to break down complex tasks into steps, reason about trade-offs, or organize your thoughts. This tool has no side effects." }
func (t *Think) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"thought": map[string]any{
				"type":        "string",
				"description": "Your step-by-step plan or reasoning",
			},
		},
		"required": []string{"thought"},
	}
}

func (t *Think) Execute(input json.RawMessage) (string, error) {
	return "Plan noted. Proceed with execution.", nil
}
