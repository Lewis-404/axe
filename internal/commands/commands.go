package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type CustomCommand struct {
	Name    string
	Content string
}

// LoadProjectCommands loads .axe/commands/*.md from the project dir
func LoadProjectCommands(dir string) []CustomCommand {
	cmdDir := filepath.Join(dir, ".axe", "commands")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return nil
	}
	var cmds []CustomCommand
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(cmdDir, e.Name()))
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		cmds = append(cmds, CustomCommand{Name: name, Content: strings.TrimSpace(string(data))})
	}
	return cmds
}

// FormatHelp returns help text for custom commands
func FormatHelp(cmds []CustomCommand) string {
	if len(cmds) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("  项目命令:\n")
	for _, c := range cmds {
		// first line as description
		desc := c.Content
		if idx := strings.Index(desc, "\n"); idx > 0 {
			desc = desc[:idx]
		}
		r := []rune(desc)
		if len(r) > 50 {
			desc = string(r[:50]) + "..."
		}
		sb.WriteString(fmt.Sprintf("  /project:%s  %s\n", c.Name, desc))
	}
	return sb.String()
}
