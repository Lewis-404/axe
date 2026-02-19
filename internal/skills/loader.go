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
			// follow symlinks
			path := filepath.Join(dir, e.Name())
			info, err := os.Stat(path)
			if err != nil || !info.IsDir() {
				continue
			}
			// try skill.yaml first, then SKILL.md with front matter
			var s Skill
			if data, err := os.ReadFile(filepath.Join(path, "skill.yaml")); err == nil {
				if yaml.Unmarshal(data, &s) != nil || s.Name == "" {
					continue
				}
			} else if data, err := os.ReadFile(filepath.Join(path, "SKILL.md")); err == nil {
				s = parseSkillMD(data)
				if s.Name == "" {
					s.Name = e.Name()
				}
			} else {
				continue
			}
			all = append(all, s)
		}
	}
	return all
}

// parseSkillMD extracts YAML front matter from SKILL.md
func parseSkillMD(data []byte) Skill {
	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		return Skill{}
	}
	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		return Skill{}
	}
	var s Skill
	yaml.Unmarshal([]byte(content[4:4+end]), &s)
	// don't load full body as system prompt — too large for 100+ skills
	return s
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

// SystemPromptExtra returns a compact skill catalog for the system prompt.
func SystemPromptExtra(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Available Skills\n")
	sb.WriteString("Use /skills to list all. Skills provide domain knowledge on demand.\n\n")
	for _, s := range skills {
		if s.Description != "" {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", s.Name, s.Description))
		} else {
			sb.WriteString(fmt.Sprintf("- **%s**\n", s.Name))
		}
	}
	return sb.String()
}
