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

// BatchConfirmItem represents a tool call pending batch confirmation.
type BatchConfirmItem struct {
	Name  string
	Input json.RawMessage
}

type Registry struct {
	tools        map[string]Tool
	confirm      func(cmd string) bool
	batchConfirm func(toolName string, items []BatchConfirmItem) bool
	postHook     PostExecHook
	skipConfirm  map[string]bool // tools to skip individual confirm (batch-approved)
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
	// wrap confirm callbacks to respect batch-approved skipConfirm
	var wrappedConfirm func(string) bool
	if opts.Confirm != nil {
		wrappedConfirm = func(cmd string) bool {
			if r.IsSkipConfirm("execute_command") || r.IsSkipConfirm("bg_command") {
				return true
			}
			return opts.Confirm(cmd)
		}
	}
	var wrappedOverwrite func(string, int, int) bool
	if opts.ConfirmOverwrite != nil {
		wrappedOverwrite = func(path string, old, new int) bool {
			if r.IsSkipConfirm("write_file") {
				return true
			}
			return opts.ConfirmOverwrite(path, old, new)
		}
	}
	var wrappedEdit func(string, string, string) bool
	if opts.ConfirmEdit != nil {
		wrappedEdit = func(path, oldText, newText string) bool {
			if r.IsSkipConfirm("edit_file") {
				return true
			}
			return opts.ConfirmEdit(path, oldText, newText)
		}
	}
	r.Register(&ReadFile{})
	r.Register(&WriteFile{confirm: wrappedOverwrite})
	r.Register(&EditFile{confirm: wrappedEdit})
	r.Register(&ListDir{})
	r.Register(&ExecCmd{confirm: wrappedConfirm})
	r.Register(&SearchFiles{})
	r.Register(&Think{})
	r.Register(&Glob{})
	r.Register(&BgCommand{confirm: wrappedConfirm})
	return r
}

func (r *Registry) SetPostExecHook(h PostExecHook)                          { r.postHook = h }
func (r *Registry) SetBatchConfirm(fn func(string, []BatchConfirmItem) bool) { r.batchConfirm = fn }

// SetSkipConfirm marks a tool to skip individual confirmation (batch-approved).
func (r *Registry) SetSkipConfirm(name string, skip bool) {
	if r.skipConfirm == nil {
		r.skipConfirm = map[string]bool{}
	}
	if skip {
		r.skipConfirm[name] = true
	} else {
		delete(r.skipConfirm, name)
	}
}

// IsSkipConfirm returns true if the tool should skip individual confirmation.
func (r *Registry) IsSkipConfirm(name string) bool {
	return r.skipConfirm != nil && r.skipConfirm[name]
}

// NeedsConfirm returns true if the tool requires user confirmation.
func (r *Registry) NeedsConfirm(name string) bool {
	switch name {
	case "write_file", "edit_file", "execute_command", "bg_command":
		return true
	}
	return false
}

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

// BatchConfirm asks user to confirm a group of same-type tool calls at once.
// Returns true if approved. Falls back to true (no batch callback set).
func (r *Registry) BatchConfirm(toolName string, items []BatchConfirmItem) bool {
	if r.batchConfirm == nil {
		return true
	}
	return r.batchConfirm(toolName, items)
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
