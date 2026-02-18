package tools

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

var dangerousPrefixes = []string{"rm -rf /", "sudo rm", "mkfs", "dd if=", "> /dev/"}

type ExecCmd struct {
	confirm func(string) bool
}

func (t *ExecCmd) Name() string        { return "execute_command" }
func (t *ExecCmd) Description() string { return "Execute a shell command" }
func (t *ExecCmd) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{"type": "string", "description": "Shell command to execute"},
		},
		"required": []string{"command"},
	}
}

func (t *ExecCmd) Execute(input json.RawMessage) (string, error) {
	var p struct{ Command string `json:"command"` }
	if err := json.Unmarshal(input, &p); err != nil {
		return "", err
	}

	for _, prefix := range dangerousPrefixes {
		if strings.HasPrefix(strings.TrimSpace(p.Command), prefix) {
			return "", fmt.Errorf("blocked dangerous command: %s", p.Command)
		}
	}

	if t.confirm != nil && !t.confirm(p.Command) {
		return "", fmt.Errorf("command rejected by user")
	}

	cmd := exec.Command("sh", "-c", p.Command)
	out, err := cmd.CombinedOutput()
	result := string(out)
	if err != nil {
		return result, fmt.Errorf("command failed: %w\noutput: %s", err, result)
	}
	return result, nil
}
