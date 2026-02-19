package ui

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

var renderer *glamour.TermRenderer

func init() {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)
	if err == nil {
		renderer = r
	}
}

// RenderMarkdown renders markdown text for terminal display.
// Falls back to plain text if rendering fails.
func RenderMarkdown(text string) string {
	if renderer == nil {
		return text
	}
	// only render if text looks like it contains markdown
	if !hasMarkdown(text) {
		return text
	}
	out, err := renderer.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimSpace(out)
}

func hasMarkdown(s string) bool {
	markers := []string{"```", "## ", "### ", "**", "| ", "- ", "1. ", "> "}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
