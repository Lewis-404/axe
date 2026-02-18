package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type WriteFile struct {
	confirm func(path string, oldLines, newLines int) bool
}

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
	if existing, err := os.ReadFile(p.Path); err == nil && t.confirm != nil {
		oldLines := strings.Count(string(existing), "\n") + 1
		newLines := strings.Count(p.Content, "\n") + 1
		if !t.confirm(p.Path, oldLines, newLines) {
			return "用户取消", nil
		}
	}
	if err := os.WriteFile(p.Path, []byte(p.Content), 0644); err != nil {
		return "", fmt.Errorf("write %s: %w", p.Path, err)
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(p.Content), p.Path), nil
}
