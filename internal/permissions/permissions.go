package permissions

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Rule struct {
	Tool    string `yaml:"tool"`
	Pattern string `yaml:"pattern"` // prefix match for command/path
	Allow   bool   `yaml:"allow"`
}

type Store struct {
	Rules []Rule `yaml:"rules"`
	path  string
}

func permFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".axe", "permissions.yaml")
}

func Load() *Store {
	s := &Store{path: permFile()}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return s
	}
	yaml.Unmarshal(data, s)
	return s
}

func (s *Store) save() error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(s.path), 0700)
	return os.WriteFile(s.path, data, 0600)
}

func (s *Store) Check(tool, value string) (allowed bool, found bool) {
	// later rules override earlier ones, so scan in reverse
	for i := len(s.Rules) - 1; i >= 0; i-- {
		r := s.Rules[i]
		if r.Tool != tool {
			continue
		}
		if r.Pattern == "*" || strings.HasPrefix(value, r.Pattern) {
			return r.Allow, true
		}
	}
	return false, false
}

func (s *Store) AddAllow(tool, pattern string) error {
	s.Rules = append(s.Rules, Rule{Tool: tool, Pattern: pattern, Allow: true})
	return s.save()
}
