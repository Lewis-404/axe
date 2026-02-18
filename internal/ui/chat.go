package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func ReadLine(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

func Confirm(cmd string) bool {
	fmt.Printf("\n⚡ Execute: %s\n", cmd)
	answer := ReadLine("Allow? [y/N] ")
	return strings.ToLower(answer) == "y"
}

func PrintAssistant(text string) {
	fmt.Printf("\n🪓 %s\n", text)
}

func PrintTool(name, input string) {
	fmt.Printf("  🔧 %s(%s)\n", name, truncate(input, 80))
}

func PrintError(err error) {
	fmt.Fprintf(os.Stderr, "\n❌ %s\n", err)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
