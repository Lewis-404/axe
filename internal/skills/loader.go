package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Skill struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Dir         string `yaml:"-"` // directory path
}

// LoadSkills scans directories for skill folders containing SKILL.md or skill.yaml
func LoadSkills(dirs ...string) []Skill {
	var all []Skill
	seen := map[string]bool{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			path := filepath.Join(dir, e.Name())
			info, err := os.Stat(path) // follow symlinks
			if err != nil || !info.IsDir() {
				continue
			}
			var s Skill
			if data, err := os.ReadFile(filepath.Join(path, "SKILL.md")); err == nil {
				s = parseFrontMatter(data)
			} else if data, err := os.ReadFile(filepath.Join(path, "skill.yaml")); err == nil {
				yaml.Unmarshal(data, &s)
			} else {
				continue
			}
			if s.Name == "" {
				s.Name = e.Name()
			}
			if seen[s.Name] {
				continue
			}
			seen[s.Name] = true
			s.Dir = path
			all = append(all, s)
		}
	}
	return all
}

func parseFrontMatter(data []byte) Skill {
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
	return s
}

// ReadSkillContent reads the full SKILL.md content for on-demand injection
func ReadSkillContent(s Skill) (string, error) {
	for _, name := range []string{"SKILL.md", "skill.yaml"} {
		data, err := os.ReadFile(filepath.Join(s.Dir, name))
		if err == nil {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("skill content not found: %s", s.Name)
}

// FindSkill finds a skill by name (exact or prefix match)
func FindSkill(skills []Skill, query string) *Skill {
	query = strings.ToLower(query)
	// exact match first
	for i := range skills {
		if strings.ToLower(skills[i].Name) == query {
			return &skills[i]
		}
	}
	// prefix match
	var match *Skill
	count := 0
	for i := range skills {
		if strings.HasPrefix(strings.ToLower(skills[i].Name), query) {
			match = &skills[i]
			count++
		}
	}
	if count == 1 {
		return match
	}
	return nil
}

// SkillCatalog returns a compact listing for system prompt
func SkillCatalog(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Available Skills\n")
	sb.WriteString("User can invoke skills with `/skill <name>`. Skills inject domain knowledge on demand.\n")
	sb.WriteString(fmt.Sprintf("Total: %d skills. Use /skills to list all.\n", len(skills)))
	return sb.String()
}
