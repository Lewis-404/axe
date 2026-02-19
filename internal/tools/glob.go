package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Glob struct{}

func (g *Glob) Name() string        { return "glob" }
func (g *Glob) Description() string { return "Search for files by name pattern (e.g. **/*.go, *.yaml). Returns matching file paths." }
func (g *Glob) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{"type": "string", "description": "Glob pattern (e.g. **/*.go, src/**/*.ts)"},
			"path":    map[string]any{"type": "string", "description": "Base directory to search in (default: current dir)"},
		},
		"required": []string{"pattern"},
	}
}

func (g *Glob) Execute(input json.RawMessage) (string, error) {
	var p struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", err
	}
	base := p.Path
	if base == "" {
		base = "."
	}

	var matches []string
	filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		// match against both full relative path and basename
		matched, _ := filepath.Match(p.Pattern, name)
		if !matched {
			matched, _ = filepath.Match(p.Pattern, path)
		}
		// support **/ prefix by matching just the suffix
		if !matched && strings.HasPrefix(p.Pattern, "**/") {
			suffix := p.Pattern[3:]
			matched, _ = filepath.Match(suffix, name)
		}
		if matched {
			matches = append(matches, path)
		}
		if len(matches) >= 200 {
			return fmt.Errorf("limit")
		}
		return nil
	})

	if len(matches) == 0 {
		return "No files matched.", nil
	}
	result := strings.Join(matches, "\n")
	if len(matches) >= 200 {
		result += "\n... (truncated at 200 results)"
	}
	return result, nil
}
