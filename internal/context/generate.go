package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateCLAUDEMD analyzes the project and generates a CLAUDE.md template
func GenerateCLAUDEMD(dir string) string {
	var sb strings.Builder
	projectName := filepath.Base(dir)

	sb.WriteString(fmt.Sprintf("# %s\n\n", projectName))
	sb.WriteString("## 项目概述\n")
	sb.WriteString("<!-- 简要描述项目功能 -->\n\n")

	// detect project type and tech stack
	sb.WriteString("## 技术栈\n")
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		data, _ := os.ReadFile(filepath.Join(dir, "go.mod"))
		lines := strings.SplitN(string(data), "\n", 3)
		if len(lines) > 0 {
			sb.WriteString(fmt.Sprintf("- %s\n", strings.TrimSpace(lines[0])))
		}
		if len(lines) > 1 {
			sb.WriteString(fmt.Sprintf("- %s\n", strings.TrimSpace(lines[1])))
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		sb.WriteString("- Node.js\n")
	}
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
		sb.WriteString("- Python\n")
	}
	if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
		sb.WriteString("- Python\n")
	}
	if _, err := os.Stat(filepath.Join(dir, "Cargo.toml")); err == nil {
		sb.WriteString("- Rust\n")
	}
	sb.WriteString("\n")

	// project structure
	sb.WriteString("## 项目结构\n```\n")
	sb.WriteString(projectName + "/\n")
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			sb.WriteString(fmt.Sprintf("├── %s/\n", name))
			// one level deep
			subs, _ := os.ReadDir(filepath.Join(dir, name))
			for _, s := range subs {
				if strings.HasPrefix(s.Name(), ".") {
					continue
				}
				if s.IsDir() {
					sb.WriteString(fmt.Sprintf("│   ├── %s/\n", s.Name()))
				} else {
					sb.WriteString(fmt.Sprintf("│   ├── %s\n", s.Name()))
				}
			}
		} else {
			sb.WriteString(fmt.Sprintf("├── %s\n", name))
		}
	}
	sb.WriteString("```\n\n")

	// common sections
	sb.WriteString("## 代码规范\n")
	sb.WriteString("- 简洁直接，不过度抽象\n")
	sb.WriteString("- 错误处理规范\n")
	sb.WriteString("- 先跑通再优化\n\n")

	sb.WriteString("## 常用命令\n")
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		sb.WriteString("```bash\ngo build ./...\ngo test ./...\n```\n")
	} else if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		sb.WriteString("```bash\nnpm install\nnpm run dev\nnpm test\n```\n")
	} else if _, err := os.Stat(filepath.Join(dir, "Cargo.toml")); err == nil {
		sb.WriteString("```bash\ncargo build\ncargo test\n```\n")
	} else if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
		sb.WriteString("```bash\npip install -r requirements.txt\npython main.py\n```\n")
	}

	return sb.String()
}
