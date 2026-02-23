package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type ReadFile struct{}

func (t *ReadFile) Name() string        { return "read_file" }
func (t *ReadFile) Description() string { return "Read the contents of a file. Use offset and limit to read specific line ranges for large files." }
func (t *ReadFile) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":   map[string]any{"type": "string", "description": "File path to read"},
			"offset": map[string]any{"type": "integer", "description": "Start line (1-indexed, default: 1)"},
			"limit":  map[string]any{"type": "integer", "description": "Max lines to read (default: all)"},
		},
		"required": []string{"path"},
	}
}

const maxReadLines = 2000

func (t *ReadFile) Execute(input json.RawMessage) (string, error) {
	var p struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", err
	}
	data, err := os.ReadFile(p.Path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", p.Path, err)
	}
	lines := strings.Split(string(data), "\n")
	total := len(lines)

	start := 0
	if p.Offset > 0 {
		start = p.Offset - 1
	}
	if start > total {
		start = total
	}

	end := total
	if p.Limit > 0 {
		end = start + p.Limit
	}
	if end-start > maxReadLines {
		end = start + maxReadLines
	}
	if end > total {
		end = total
	}

	result := strings.Join(lines[start:end], "\n")
	if end < total {
		result += fmt.Sprintf("\n... (%d more lines, use offset=%d to continue)", total-end, end+1)
	}
	return result, nil
}
