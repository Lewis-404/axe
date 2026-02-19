package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/chzyer/readline"
)

// SlashCmd defines a slash command for hints.
type SlashCmd struct {
	Name string
	Desc string
}

var slashCommands = []SlashCmd{
	{"/clear", "清空对话上下文并清屏"},
	{"/list", "查看最近对话记录"},
	{"/resume", "恢复对话（可加编号）"},
	{"/model", "查看/切换模型"},
	{"/cost", "显示累计 token 用量"},
	{"/help", "显示帮助"},
	{"/exit", "退出 Axe"},
}

// slashHinter implements readline.Listener for real-time command hints.
type slashHinter struct {
	hintLines int
}

func (h *slashHinter) OnChange(line []rune, pos int, key rune) ([]rune, int, bool) {
	// Clear previous hints
	h.clearHints()

	// line already reflects the current buffer state (character already added/removed)
	s := string(line)

	if !strings.HasPrefix(s, "/") || len(s) < 2 || strings.Contains(s, " ") {
		return line, pos, false
	}

	var matches []SlashCmd
	for _, cmd := range slashCommands {
		if strings.HasPrefix(cmd.Name, s) {
			matches = append(matches, cmd)
		}
	}

	if len(matches) > 0 {
		var buf strings.Builder
		buf.WriteString("\033[s") // save cursor
		for _, m := range matches {
			buf.WriteString(fmt.Sprintf("\n  \033[36m%s\033[0m  \033[90m%s\033[0m", m.Name, m.Desc))
		}
		buf.WriteString("\033[u") // restore cursor
		os.Stdout.WriteString(buf.String())
		h.hintLines = len(matches)
	}

	return line, pos, false
}

func (h *slashHinter) clearHints() {
	if h.hintLines == 0 {
		return
	}
	var buf strings.Builder
	buf.WriteString("\033[s") // save cursor
	for i := 0; i < h.hintLines; i++ {
		buf.WriteString("\n\033[2K") // move down + clear line
	}
	buf.WriteString("\033[u") // restore cursor
	os.Stdout.WriteString(buf.String())
	h.hintLines = 0
}

// ClearSlashHints clears any remaining hint lines (call before output).
func ClearSlashHints() {
	if hinter != nil {
		hinter.clearHints()
	}
}

var rl *readline.Instance
var hinter *slashHinter

func init() {
	hinter = &slashHinter{}

	var completer []readline.PrefixCompleterInterface
	for _, cmd := range slashCommands {
		completer = append(completer, readline.PcItem(cmd.Name))
	}

	var err error
	rl, err = readline.NewEx(&readline.Config{
		Prompt:          "you> ",
		InterruptPrompt: "^C",
		EOFPrompt:       "quit",
		AutoComplete:    readline.NewPrefixCompleter(completer...),
		Listener:        hinter,
	})
	if err != nil {
		panic(err)
	}
}

func ReadLine(prompt string) string {
	rl.SetPrompt(prompt)
	line, err := rl.Readline()
	if err != nil {
		return ""
	}
	// Clear hints after Enter
	hinter.clearHints()
	return strings.TrimSpace(line)
}

func Confirm(cmd string) bool {
	fmt.Printf("\n⚡ Execute: %s\n", cmd)
	answer := ReadLine("Allow? [y/N] ")
	return strings.ToLower(answer) == "y"
}

func ConfirmOverwrite(path string, oldLines, newLines int) bool {
	fmt.Printf("\n📝 覆盖 %s (原 %d 行 → 新 %d 行)\n", path, oldLines, newLines)
	return strings.ToLower(ReadLine("Allow? [y/N] ")) == "y"
}

func ConfirmEdit(path, oldText, newText string) bool {
	fmt.Printf("\n✏️ 编辑 %s:\n  - %s\n  + %s\n", path, truncate(oldText, 30), truncate(newText, 30))
	return strings.ToLower(ReadLine("Allow? [y/N] ")) == "y"
}

var streamStarted bool

func PrintTextDelta(text string) {
	if !streamStarted {
		fmt.Print("\n🪓 ")
		streamStarted = true
	}
	fmt.Print(text)
}

func PrintBlockDone() {
	if streamStarted {
		fmt.Println()
		streamStarted = false
	}
}

func PrintAssistant(text string) {
	fmt.Printf("\n🪓 %s\n", text)
}

func PrintTool(name, input string) {
	fmt.Printf("  🔧 %s(%s)\n", name, truncate(input, 80))
}

func PrintUsage(roundIn, roundOut, totalIn, totalOut int) {
	fmt.Printf("📊 本轮: ↑%s ↓%s | 累计: ↑%s ↓%s\n", fmtTokens(roundIn), fmtTokens(roundOut), fmtTokens(totalIn), fmtTokens(totalOut))
}

func fmtTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func PrintTotalUsage(totalIn, totalOut int) {
	fmt.Printf("📊 累计: ↑%s ↓%s\n", fmtTokens(totalIn), fmtTokens(totalOut))
}

func PrintError(err error) {
	fmt.Fprintf(os.Stderr, "\n❌ %s\n", err)
}

func ClearScreen() {
	fmt.Print("\033[2J\033[H")
}

func CloseRL() {
	if rl != nil {
		rl.Close()
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
