package tools

import (
	"encoding/json"
	"fmt"
	"os"
)

type ReadFile struct{}

func (t *ReadFile) Name() string        { return "read_file" }
func (t *ReadFile) Description() string { return "Read the contents of a file" }
func (t *ReadFile) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "File path to read"},
		},
		"required": []string{"path"},
	}
}

func (t *ReadFile) Execute(input json.RawMessage) (string, error) {
	var p struct{ Path string `json:"path"` }
	if err := json.Unmarshal(input, &p); err != nil {
		return "", err
	}
	data, err := os.ReadFile(p.Path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", p.Path, err)
	}
	return string(data), nil
}
