package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const maxBgOutput = 64 * 1024 // 64KB max per process

// cappedBuffer is a bytes.Buffer that discards old data when exceeding maxSize.
type cappedBuffer struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	maxSize int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buf.Write(p)
	if c.buf.Len() > c.maxSize {
		// keep only the last maxSize bytes
		b := c.buf.Bytes()
		c.buf.Reset()
		c.buf.Write(b[len(b)-c.maxSize:])
	}
	return len(p), nil
}

func (c *cappedBuffer) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

type bgProc struct {
	ID      int
	Cmd     string
	Process *exec.Cmd
	Output  *cappedBuffer
	Started time.Time
	Done    bool
	Err     error
}

var (
	bgMu    sync.Mutex
	bgProcs []*bgProc
	bgSeq   int
)

type BgCommand struct {
	confirm func(string) bool
}

func (t *BgCommand) Name() string { return "bg_command" }
func (t *BgCommand) Description() string {
	return "Start a background process (e.g. dev server). Use action=start to launch, action=status to check, action=stop to kill, action=logs to read output."
}
func (t *BgCommand) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action":  map[string]any{"type": "string", "enum": []string{"start", "status", "stop", "logs"}, "description": "Action to perform"},
			"command": map[string]any{"type": "string", "description": "Shell command (for start)"},
			"id":      map[string]any{"type": "integer", "description": "Process ID (for stop/logs)"},
		},
		"required": []string{"action"},
	}
}

func (t *BgCommand) Execute(input json.RawMessage) (string, error) {
	var p struct {
		Action  string `json:"action"`
		Command string `json:"command"`
		ID      int    `json:"id"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", err
	}

	switch p.Action {
	case "start":
		if p.Command == "" {
			return "", fmt.Errorf("command is required for start")
		}
		for _, prefix := range dangerousPrefixes {
			if strings.HasPrefix(strings.TrimSpace(p.Command), prefix) {
				return "", fmt.Errorf("blocked dangerous command: %s", p.Command)
			}
		}
		if t.confirm != nil && !t.confirm(p.Command) {
			return "", fmt.Errorf("command rejected by user")
		}
		buf := &cappedBuffer{maxSize: maxBgOutput}
		cmd := exec.Command("sh", "-c", p.Command)
		cmd.Stdout = buf
		cmd.Stderr = buf
		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("start failed: %w", err)
		}
		bgMu.Lock()
		bgSeq++
		proc := &bgProc{ID: bgSeq, Cmd: p.Command, Process: cmd, Output: buf, Started: time.Now()}
		bgProcs = append(bgProcs, proc)
		bgMu.Unlock()
		go func() {
			err := cmd.Wait()
			bgMu.Lock()
			proc.Done = true
			proc.Err = err
			bgMu.Unlock()
		}()
		return fmt.Sprintf("Started background process [%d]: %s", proc.ID, p.Command), nil

	case "status":
		bgMu.Lock()
		defer bgMu.Unlock()
		if len(bgProcs) == 0 {
			return "No background processes.", nil
		}
		var lines []string
		for _, proc := range bgProcs {
			status := "running"
			if proc.Done {
				status = "stopped"
				if proc.Err != nil {
					status = fmt.Sprintf("exited (%s)", proc.Err)
				}
			}
			lines = append(lines, fmt.Sprintf("[%d] %s — %s (since %s)", proc.ID, proc.Cmd, status, proc.Started.Format("15:04:05")))
		}
		return fmt.Sprintf("%d processes:\n%s", len(bgProcs), joinLines(lines)), nil

	case "stop":
		bgMu.Lock()
		proc := findProc(p.ID)
		bgMu.Unlock()
		if proc == nil {
			return "", fmt.Errorf("process [%d] not found", p.ID)
		}
		if proc.Done {
			return fmt.Sprintf("Process [%d] already stopped.", p.ID), nil
		}
		proc.Process.Process.Kill()
		return fmt.Sprintf("Killed process [%d]: %s", p.ID, proc.Cmd), nil

	case "logs":
		bgMu.Lock()
		proc := findProc(p.ID)
		bgMu.Unlock()
		if proc == nil {
			return "", fmt.Errorf("process [%d] not found", p.ID)
		}
		out := proc.Output.String()
		if out == "" {
			out = "(no output yet)"
		}
		return out, nil

	default:
		return "", fmt.Errorf("unknown action: %s", p.Action)
	}
}

func findProc(id int) *bgProc {
	for _, p := range bgProcs {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}
