package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Lewis-404/axe/internal/tools"
	"gopkg.in/yaml.v3"
)

type ToolDef struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Parameters  map[string]Param  `yaml:"parameters"`
	Command     string            `yaml:"command"`
}

type Param struct {
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
}

type Skill struct {
	Name         string    `yaml:"name"`
	Description  string    `yaml:"description"`
	SystemPrompt string    `yaml:"system_prompt"`
	Tools        []ToolDef `yaml:"tools"`
}

func LoadSkills(dirs ...string) []Skill {
	var all []Skill
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			path := filepath.Join(dir, e.Name(), "skill.yaml")
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var s Skill
			if yaml.Unmarshal(data, &s) == nil && s.Name != "" {
				all = append(all, s)
			}
		}
	}
	return all
}

// SkillTool wraps a skill's command-based tool definition as a tools.Tool.
type SkillTool struct {
	Def     ToolDef
	Confirm func(string) bool
}

func (t *SkillTool) Name() string        { return t.Def.Name }
func (t *SkillTool) Description() string { return t.Def.Description }
func (t *SkillTool) Schema() any {
	props := map[string]any{}
	var required []string
	for k, v := range t.Def.Parameters {
		props[k] = map[string]any{"type": v.Type, "description": v.Description}
		required = append(required, k)
	}
	return map[string]any{"type": "object", "properties": props, "required": required}
}

func (t *SkillTool) Execute(input json.RawMessage) (string, error) {
	var params map[string]string
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}
	cmdStr := t.Def.Command
	for k, v := range params {
		cmdStr = strings.ReplaceAll(cmdStr, "{{"+k+"}}", v)
	}
	if t.Confirm != nil && !t.Confirm(cmdStr) {
		return "", fmt.Errorf("command rejected by user")
	}
	cmd := exec.Command("sh", "-c", cmdStr)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("command failed: %w\noutput: %s", err, string(out))
	}
	return string(out), nil
}

// RegisterTools registers all command-based tools from skills into the registry.
func RegisterTools(skills []Skill, registry *tools.Registry, confirm func(string) bool) {
	for _, s := range skills {
		for _, td := range s.Tools {
			registry.Register(&SkillTool{Def: td, Confirm: confirm})
		}
	}
}

// SystemPromptExtra returns combined system prompt additions from all skills.
func SystemPromptExtra(skills []Skill) string {
	var parts []string
	for _, s := range skills {
		if s.SystemPrompt != "" {
			parts = append(parts, s.SystemPrompt)
		}
	}
	return strings.Join(parts, "\n\n")
}
