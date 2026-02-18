package tools

import (
	"encoding/json"
	"os/exec"
	"strings"
)

type SearchFiles struct{}

func (t *SearchFiles) Name() string        { return "search_files" }
func (t *SearchFiles) Description() string { return "Search for a pattern in files using grep" }
func (t *SearchFiles) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{"type": "string", "description": "Search pattern (regex)"},
			"path":    map[string]any{"type": "string", "description": "Directory to search in"},
		},
		"required": []string{"pattern", "path"},
	}
}

func (t *SearchFiles) Execute(input json.RawMessage) (string, error) {
	var p struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", err
	}
	cmd := exec.Command("grep", "-rn", "--include=*.go", "--include=*.yaml", "--include=*.yml",
		"--include=*.json", "--include=*.md", "--include=*.txt", "--include=*.mod",
		"-I", p.Pattern, p.Path)
	out, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if err != nil {
		if result == "" {
			return "(no matches)", nil
		}
		return result, nil
	}
	lines := strings.Split(result, "\n")
	if len(lines) > 50 {
		lines = lines[:50]
		result = strings.Join(lines, "\n") + "\n... (truncated, 50+ matches)"
	}
	return result, nil
}
