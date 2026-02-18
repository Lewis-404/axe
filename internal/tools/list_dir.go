package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ListDir struct{}

func (t *ListDir) Name() string        { return "list_directory" }
func (t *ListDir) Description() string { return "List files and directories in a path" }
func (t *ListDir) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "Directory path to list"},
		},
		"required": []string{"path"},
	}
}

func (t *ListDir) Execute(input json.RawMessage) (string, error) {
	var p struct{ Path string `json:"path"` }
	if err := json.Unmarshal(input, &p); err != nil {
		return "", err
	}

	var lines []string
	err := filepath.WalkDir(p.Path, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(p.Path, path)
		if rel == "." {
			return nil
		}
		// skip hidden dirs and common noise
		name := d.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		depth := strings.Count(rel, string(os.PathSeparator))
		if depth > 3 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		prefix := strings.Repeat("  ", depth)
		if d.IsDir() {
			lines = append(lines, fmt.Sprintf("%s%s/", prefix, name))
		} else {
			lines = append(lines, fmt.Sprintf("%s%s", prefix, name))
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk %s: %w", p.Path, err)
	}
	if len(lines) == 0 {
		return "(empty directory)", nil
	}
	return strings.Join(lines, "\n"), nil
}
