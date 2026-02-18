package tools

import (
	"encoding/json"
	"fmt"

	"github.com/Lewis-404/axe/internal/llm"
)

type Tool interface {
	Name() string
	Description() string
	Schema() any
	Execute(input json.RawMessage) (string, error)
}

type Registry struct {
	tools map[string]Tool
	confirm func(cmd string) bool
}

func NewRegistry(confirm func(string) bool) *Registry {
	r := &Registry{tools: make(map[string]Tool), confirm: confirm}
	r.Register(&ReadFile{})
	r.Register(&WriteFile{})
	r.Register(&EditFile{})
	r.Register(&ListDir{})
	r.Register(&ExecCmd{confirm: confirm})
	r.Register(&SearchFiles{})
	return r
}

func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

func (r *Registry) Execute(name string, input json.RawMessage) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return t.Execute(input)
}

func (r *Registry) Definitions() []llm.ToolDef {
	var defs []llm.ToolDef
	for _, t := range r.tools {
		defs = append(defs, llm.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.Schema(),
		})
	}
	return defs
}
