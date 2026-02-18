package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type WriteFile struct{}

func (t *WriteFile) Name() string        { return "write_file" }
func (t *WriteFile) Description() string { return "Create or overwrite a file with content" }
func (t *WriteFile) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "File path to write"},
			"content": map[string]any{"type": "string", "description": "Content to write"},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteFile) Execute(input json.RawMessage) (string, error) {
	var p struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p.Path), 0755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(p.Path, []byte(p.Content), 0644); err != nil {
		return "", fmt.Errorf("write %s: %w", p.Path, err)
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(p.Content), p.Path), nil
}
