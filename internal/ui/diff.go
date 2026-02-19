package ui

import (
	"fmt"
	"strings"
)

// PrintDiff prints a colored unified diff
func PrintDiff(path, oldText, newText string) {
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")

	fmt.Printf("\033[1m--- %s\033[0m\n", path)
	fmt.Printf("\033[1m+++ %s\033[0m\n", path)

	// simple line-by-line diff
	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	i, j := 0, 0
	for i < len(oldLines) || j < len(newLines) {
		if i < len(oldLines) && j < len(newLines) && oldLines[i] == newLines[j] {
			fmt.Printf(" %s\n", oldLines[i])
			i++
			j++
		} else if i < len(oldLines) && (j >= len(newLines) || !containsFrom(newLines, j, oldLines[i])) {
			fmt.Printf("\033[31m-%s\033[0m\n", oldLines[i])
			i++
		} else if j < len(newLines) {
			fmt.Printf("\033[32m+%s\033[0m\n", newLines[j])
			j++
		}
	}
}

func containsFrom(lines []string, start int, target string) bool {
	end := start + 5
	if end > len(lines) {
		end = len(lines)
	}
	for k := start; k < end; k++ {
		if lines[k] == target {
			return true
		}
	}
	return false
}
