package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func loadPatterns(dir string, filename string) []string {
	data, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		return nil
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

func isIgnored(name string, patterns []string) bool {
	for _, p := range patterns {
		if matched, _ := filepath.Match(p, name); matched {
			return true
		}
	}
	return false
}

// detectKeyFiles returns key files to read based on what exists in dir.
func detectKeyFiles(dir string) []string {
	type keyFile struct {
		name     string
		maxLines int // 0 = read full file (up to 2000 chars)
	}

	candidates := []keyFile{
		// Go
		{"go.mod", 0}, {"main.go", 0}, {"go.sum", 20},
		// Python
		{"requirements.txt", 0}, {"setup.py", 0}, {"pyproject.toml", 0}, {"main.py", 0}, {"app.py", 0},
		// Node
		{"package.json", 0}, {"tsconfig.json", 0},
		// Rust
		{"Cargo.toml", 0},
		// Generic
		{"README.md", 0}, {"Makefile", 0}, {"Dockerfile", 0}, {"docker-compose.yml", 0},
	}

	var result []string
	for _, kf := range candidates {
		path := filepath.Join(dir, kf.name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		if kf.maxLines > 0 {
			lines := strings.SplitN(content, "\n", kf.maxLines+1)
			if len(lines) > kf.maxLines {
				lines = lines[:kf.maxLines]
			}
			content = strings.Join(lines, "\n") + "\n... (first 20 lines)"
		} else if len(content) > 2000 {
			content = content[:2000] + "\n... (truncated)"
		}
		result = append(result, fmt.Sprintf("\n--- %s ---\n%s\n", kf.name, content))
	}
	return result
}

const maxFileSize = 100 * 1024 // 100KB

func Collect(dir string) string {
	var sb strings.Builder

	// CLAUDE.md / .axe.md project instructions
	for _, name := range []string{"CLAUDE.md", ".axe.md"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("--- CLAUDE.md (project instructions) ---\n%s\n\n", strings.TrimSpace(string(data))))
		break
	}

	sb.WriteString(fmt.Sprintf("Project directory: %s\n\n", dir))

	// load ignore patterns from .axeignore and .gitignore
	ignorePatterns := loadPatterns(dir, ".axeignore")
	ignorePatterns = append(ignorePatterns, loadPatterns(dir, ".gitignore")...)

	// file tree (max depth 3)
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
		if isIgnored(name, ignorePatterns) {
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
		if depth > 3 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// skip files > 100KB
		if !d.IsDir() {
			if info, err := d.Info(); err == nil && info.Size() > maxFileSize {
				return nil
			}
		}
		prefix := strings.Repeat("  ", depth)
		if d.IsDir() {
			sb.WriteString(fmt.Sprintf("%s%s/\n", prefix, name))
		} else {
			sb.WriteString(fmt.Sprintf("%s%s\n", prefix, name))
		}
		return nil
	})

	// smart key file detection
	for _, section := range detectKeyFiles(dir) {
		sb.WriteString(section)
	}

	return sb.String()
}
