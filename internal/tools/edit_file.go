package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type EditFile struct{}

func (t *EditFile) Name() string        { return "edit_file" }
func (t *EditFile) Description() string { return "Replace exact text in a file" }
func (t *EditFile) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":     map[string]any{"type": "string", "description": "File path"},
			"old_text": map[string]any{"type": "string", "description": "Exact text to find"},
			"new_text": map[string]any{"type": "string", "description": "Replacement text"},
		},
		"required": []string{"path", "old_text", "new_text"},
	}
}

func (t *EditFile) Execute(input json.RawMessage) (string, error) {
	var p struct {
		Path    string `json:"path"`
		OldText string `json:"old_text"`
		NewText string `json:"new_text"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", err
	}
	data, err := os.ReadFile(p.Path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", p.Path, err)
	}
	content := string(data)
	if !strings.Contains(content, p.OldText) {
		return "", fmt.Errorf("old_text not found in %s", p.Path)
	}
	content = strings.Replace(content, p.OldText, p.NewText, 1)
	if err := os.WriteFile(p.Path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write %s: %w", p.Path, err)
	}
	return fmt.Sprintf("edited %s", p.Path), nil
}
