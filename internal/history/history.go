package history

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Lewis-404/axe/internal/llm"
)

type Record struct {
	CreatedAt  string        `json:"created_at"`
	UpdatedAt  string        `json:"updated_at"`
	ProjectDir string        `json:"project_dir,omitempty"`
	Messages   []llm.Message `json:"messages"`
}

var projectDir string // set by caller via SetProjectDir

func SetProjectDir(dir string) { projectDir = dir }

func projectHash(dir string) string {
	h := sha256.Sum256([]byte(dir))
	return fmt.Sprintf("%x", h[:8])
}

func projectSlug(dir string) string {
	name := filepath.Base(dir)
	// sanitize: keep only alphanumeric, dash, underscore
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		}
	}
	slug := sb.String()
	if slug == "" {
		slug = "default"
	}
	return slug + "-" + projectHash(dir)[:8]
}

func historyDir() string {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".axe", "history")
	if projectDir == "" {
		return base
	}
	return filepath.Join(base, projectSlug(projectDir))
}

// ensureDir creates history dir and writes a .project meta file
func ensureDir() error {
	dir := historyDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if projectDir != "" {
		meta := filepath.Join(dir, ".project")
		os.WriteFile(meta, []byte(projectDir), 0600)
	}
	return nil
}

func SaveTo(path string, messages []llm.Message) error {
	rec := &Record{
		UpdatedAt:  time.Now().Format("2006-01-02_150405"),
		ProjectDir: projectDir,
		Messages:   messages,
	}
	if data, err := os.ReadFile(path); err == nil {
		var existing Record
		if json.Unmarshal(data, &existing) == nil {
			rec.CreatedAt = existing.CreatedAt
		}
	} else {
		rec.CreatedAt = rec.UpdatedAt
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal history: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

func listFiles() ([]string, error) {
	dir := historyDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read history dir: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

func LoadLatest() (string, []llm.Message, error) {
	files, err := listFiles()
	if err != nil {
		return "", nil, err
	}
	if len(files) == 0 {
		return "", nil, fmt.Errorf("no history found")
	}
	path := files[len(files)-1]
	msgs, err := loadFile(path)
	return path, msgs, err
}

func loadFile(path string) ([]llm.Message, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read history file: %w", err)
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("parse history file: %w", err)
	}
	return rec.Messages, nil
}

func NewFilePath() string {
	ensureDir()
	now := time.Now().Format("2006-01-02_150405")
	return filepath.Join(historyDir(), now+".json")
}

func LoadByIndex(idx int) (string, []llm.Message, error) {
	files, err := listFiles()
	if err != nil {
		return "", nil, err
	}
	if len(files) == 0 {
		return "", nil, fmt.Errorf("no history found")
	}
	if idx < 1 || idx > len(files) {
		return "", nil, fmt.Errorf("invalid index %d (1-%d)", idx, len(files))
	}
	path := files[idx-1]
	msgs, err := loadFile(path)
	return path, msgs, err
}

func ListRecentIndexed(n int) ([]string, error) {
	files, err := listFiles()
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return []string{"No history found."}, nil
	}
	start := 0
	if len(files) > n {
		start = len(files) - n
	}
	var lines []string
	for i, f := range files[start:] {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var rec Record
		if json.Unmarshal(data, &rec) != nil {
			continue
		}
		summary := "(empty)"
		for _, m := range rec.Messages {
			if m.Role == llm.RoleUser {
				for _, b := range m.Content {
					if b.Type == "text" && b.Text != "" {
						r := []rune(b.Text)
						if len(r) > 50 {
							summary = string(r[:50]) + "..."
						} else {
							summary = b.Text
						}
						goto found
					}
				}
			}
		}
	found:
		idx := start + i + 1
		name := filepath.Base(f)
		ts := name[:len(name)-len(".json")]
		lines = append(lines, fmt.Sprintf("  [%d] %s  %s", idx, ts, summary))
	}
	return lines, nil
}
