package ui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Lewis-404/axe/internal/llm"
	"github.com/nyaosorg/go-box/v3"
	"github.com/nyaosorg/go-readline-ny"
	"github.com/nyaosorg/go-readline-ny/keys"
	"golang.org/x/term"
)

var editor *readline.Editor

type slashCmd struct {
	name string
	desc string
}

var slashCommands = []slashCmd{
	{"/clear", "清空对话上下文并清屏"},
	{"/compact", "压缩对话上下文（可加提示）"},
	{"/fork", "从当前对话创建分支"},
	{"/init", "为当前项目生成 CLAUDE.md"},
	{"/list", "查看最近对话记录"},
	{"/resume", "恢复对话（可加编号）"},
	{"/model", "查看/切换模型"},
	{"/ask", "临时用另一个模型回答"},
	{"/search", "搜索历史对话"},
	{"/undo", "撤销上一次 git commit"},
	{"/diff", "查看未提交的变更"},
	{"/retry", "重试上一轮对话"},
	{"/export", "导出对话为 Markdown"},
	{"/git", "快捷 git 操作 (status/log/branch)"},
	{"/context", "查看上下文 token 用量"},
	{"/skills", "查看已加载的技能"},
	{"/skill", "激活技能 (/skill <name>)"},
	{"/budget", "设置费用上限"},
	{"/cost", "显示累计 token 用量和费用"},
	{"/help", "显示帮助"},
	{"/exit", "退出 Axe"},
}

// RegisterSkillCommands adds skills as slash commands
func RegisterSkillCommands(names []string, descs []string) {
	for i, name := range names {
		desc := ""
		if i < len(descs) {
			desc = descs[i]
		}
		slashCommands = append(slashCommands, slashCmd{"/" + name, desc})
	}
}

func init() {
	editor = &readline.Editor{
		PromptWriter: func(w io.Writer) (int, error) {
			return io.WriteString(w, "\033[36m❯\033[0m ")
		},
		Writer: os.Stdout,
	}

	// Tab: complete slash commands, double-tab shows list via go-box
	editor.BindKey(keys.CtrlI, readline.AnonymousCommand(func(ctx context.Context, b *readline.Buffer) readline.Result {
		line := b.String()
		if !strings.HasPrefix(line, "/") {
			return readline.CONTINUE
		}

		var matches []string
		for _, cmd := range slashCommands {
			if strings.HasPrefix(cmd.name, line) {
				matches = append(matches, cmd.name)
			}
		}
		if len(matches) == 0 {
			return readline.CONTINUE
		}
		if len(matches) == 1 {
			b.ReplaceAndRepaint(0, matches[0]+" ")
			return readline.CONTINUE
		}

		// find common prefix
		prefix := matches[0]
		for _, m := range matches[1:] {
			for !strings.HasPrefix(m, prefix) {
				prefix = prefix[:len(prefix)-1]
			}
		}
		if prefix != line {
			b.ReplaceAndRepaint(0, prefix)
			return readline.CONTINUE
		}

		// show list with descriptions
		b.Out.WriteByte('\n')
		var display []string
		for _, m := range matches {
			desc := ""
			for _, cmd := range slashCommands {
				if cmd.name == m {
					desc = cmd.desc
					break
				}
			}
			if desc != "" {
				display = append(display, fmt.Sprintf("%-28s %s", m, desc))
			} else {
				display = append(display, m)
			}
		}
		box.Println(display, b.Out)
		b.RepaintAll()
		return readline.CONTINUE
	}))
}

func ReadLine(prompt string) string {
	editor.PromptWriter = func(w io.Writer) (int, error) {
		return io.WriteString(w, prompt)
	}
	line, err := editor.ReadLine(context.Background())
	if err != nil {
		return ""
	}
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
var streamBuf strings.Builder

func getTermWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// displayLines calculates actual terminal lines considering wrapping.
func displayLines(text string, width int) int {
	lines := 0
	for _, line := range strings.Split(text, "\n") {
		r := []rune(line)
		if len(r) == 0 {
			lines++
		} else {
			lines += (len(r) + width - 1) / width
		}
	}
	return lines
}

func PrintTextDelta(text string) {
	if !streamStarted {
		fmt.Print("\n")
		streamStarted = true
	}
	streamBuf.WriteString(text)
	fmt.Print(text)
}

func PrintBlockDone() {
	if streamStarted {
		raw := streamBuf.String()
		rendered := RenderMarkdown(raw)
		if rendered != raw {
			lines := displayLines(raw, getTermWidth())
			for i := 0; i < lines; i++ {
				fmt.Print("\033[A\033[2K")
			}
			fmt.Printf("\n🪓 %s\n", rendered)
		} else {
			fmt.Println()
		}
		streamBuf.Reset()
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
	// no-op for go-readline-ny
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

// PrintHistory displays conversation messages after resume.
func PrintHistory(msgs []llm.Message) {
	if len(msgs) == 0 {
		return
	}
	fmt.Println("\033[90m── 对话历史 ──\033[0m")
	for _, m := range msgs {
		for _, b := range m.Content {
			if b.Type == "text" && b.Text != "" {
				if m.Role == llm.RoleUser {
					fmt.Printf("\033[36m❯\033[0m %s\n\n", b.Text)
				} else {
					fmt.Printf("🪓 %s\n\n", b.Text)
				}
			}
		}
	}
	fmt.Println("\033[90m── 以上为历史 ──\033[0m")
	fmt.Println()
}
