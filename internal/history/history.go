package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Lewis-404/axe/internal/llm"
)

type Record struct {
	CreatedAt string       `json:"created_at"`
	UpdatedAt string       `json:"updated_at"`
	Messages  []llm.Message `json:"messages"`
}

func historyDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".axe", "history")
}

func Save(messages []llm.Message) error {
	dir := historyDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create history dir: %w", err)
	}

	now := time.Now().Format("2006-01-02_150405")
	path := filepath.Join(dir, now+".json")

	// If file exists (resuming), update it; otherwise create new
	rec := &Record{
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  messages,
	}

	// Try to read existing to preserve CreatedAt
	if data, err := os.ReadFile(path); err == nil {
		var existing Record
		if json.Unmarshal(data, &existing) == nil {
			rec.CreatedAt = existing.CreatedAt
		}
	}

	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal history: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

func SaveTo(path string, messages []llm.Message) error {
	rec := &Record{
		UpdatedAt: time.Now().Format("2006-01-02_150405"),
		Messages:  messages,
	}

	// Preserve CreatedAt from existing file
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

func ListRecent(n int) ([]string, error) {
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
	for _, f := range files[start:] {
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
						summary = b.Text
						if len(summary) > 60 {
							summary = summary[:60] + "..."
						}
						goto found
					}
				}
			}
		}
	found:
		name := filepath.Base(f)
		ts := name[:len(name)-len(".json")]
		lines = append(lines, fmt.Sprintf("  %s  %s", ts, summary))
	}
	return lines, nil
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
	dir := historyDir()
	os.MkdirAll(dir, 0700)
	now := time.Now().Format("2006-01-02_150405")
	return filepath.Join(dir, now+".json")
}
