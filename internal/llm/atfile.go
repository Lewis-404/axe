package llm

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var atFileRe = regexp.MustCompile(`@(~?[\w./_-]+\.\w+)`)

// ExpandAtFiles replaces @filepath references with file contents
func ExpandAtFiles(input string) string {
	matches := atFileRe.FindAllStringSubmatch(input, -1)
	if len(matches) == 0 {
		return input
	}
	result := input
	for _, m := range matches {
		path := expandHome(m[1])
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		replacement := fmt.Sprintf("\n<file path=%q>\n%s\n</file>", m[1], strings.TrimRight(string(data), "\n"))
		result = strings.Replace(result, m[0], replacement, 1)
		fmt.Printf("📎 已引用 %s\n", m[1])
	}
	return result
}
