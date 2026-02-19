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
	tools    map[string]Tool
	confirm  func(cmd string) bool
	postHook PostExecHook
}

type RegistryOpts struct {
	Confirm          func(string) bool
	ConfirmOverwrite func(path string, oldLines, newLines int) bool
	ConfirmEdit      func(path, oldText, newText string) bool
}

// PostExecHook is called after a tool executes successfully. name is the tool name, result is the output.
type PostExecHook func(name string, input json.RawMessage, result string) string

func NewRegistry(opts RegistryOpts) *Registry {
	r := &Registry{tools: make(map[string]Tool), confirm: opts.Confirm}
	r.Register(&ReadFile{})
	r.Register(&WriteFile{confirm: opts.ConfirmOverwrite})
	r.Register(&EditFile{confirm: opts.ConfirmEdit})
	r.Register(&ListDir{})
	r.Register(&ExecCmd{confirm: opts.Confirm})
	r.Register(&SearchFiles{})
	r.Register(&Think{})
	return r
}

func (r *Registry) SetPostExecHook(h PostExecHook) { r.postHook = h }

func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

func (r *Registry) Execute(name string, input json.RawMessage) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	result, err := t.Execute(input)
	if err == nil && r.postHook != nil {
		if extra := r.postHook(name, input, result); extra != "" {
			result += "\n\n" + extra
		}
	}
	return result, err
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
