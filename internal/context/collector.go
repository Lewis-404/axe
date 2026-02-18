package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Collect(dir string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Project directory: %s\n\n", dir))

	// file tree
	sb.WriteString("File tree:\n")
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if rel == "." {
			return nil
		}
		depth := strings.Count(rel, string(os.PathSeparator))
		if depth > 2 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		prefix := strings.Repeat("  ", depth)
		if d.IsDir() {
			sb.WriteString(fmt.Sprintf("%s%s/\n", prefix, name))
		} else {
			sb.WriteString(fmt.Sprintf("%s%s\n", prefix, name))
		}
		return nil
	})

	// read key files
	keyFiles := []string{"go.mod", "README.md", "Makefile", "main.go"}
	for _, f := range keyFiles {
		path := filepath.Join(dir, f)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		if len(content) > 2000 {
			content = content[:2000] + "\n... (truncated)"
		}
		sb.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", f, content))
	}

	return sb.String()
}
