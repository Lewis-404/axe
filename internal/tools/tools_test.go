package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkipDir(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{".git", true},
		{"node_modules", true},
		{"vendor", true},
		{"__pycache__", true},
		{".next", true},
		{"dist", true},
		{"build", true},
		{".svn", true},
		{".hidden", true},
		{"src", false},
		{"main.go", false},
		{"README.md", false},
	}
	for _, c := range cases {
		if got := SkipDir(c.name); got != c.want {
			t.Errorf("SkipDir(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCappedBuffer(t *testing.T) {
	buf := &cappedBuffer{maxSize: 32}
	// write more than maxSize
	buf.Write([]byte(strings.Repeat("a", 50)))
	if got := buf.String(); len(got) != 32 {
		t.Errorf("cappedBuffer len = %d, want 32", len(got))
	}
	// write small amount
	buf2 := &cappedBuffer{maxSize: 100}
	buf2.Write([]byte("hello"))
	if got := buf2.String(); got != "hello" {
		t.Errorf("cappedBuffer = %q, want %q", got, "hello")
	}
}

func TestCappedBufferKeepsLatest(t *testing.T) {
	buf := &cappedBuffer{maxSize: 10}
	buf.Write([]byte("0123456789"))
	buf.Write([]byte("ABCDE"))
	got := buf.String()
	if got != "56789ABCDE" {
		t.Errorf("cappedBuffer = %q, want %q", got, "56789ABCDE")
	}
}

func TestDangerousPrefixes(t *testing.T) {
	for _, cmd := range []string{"rm -rf /", "sudo rm foo", "mkfs.ext4 /dev/sda", "dd if=/dev/zero", ":(){ :|:", "shutdown -h now"} {
		blocked := false
		for _, prefix := range dangerousPrefixes {
			if strings.HasPrefix(strings.TrimSpace(cmd), prefix) {
				blocked = true
				break
			}
		}
		if !blocked {
			t.Errorf("command %q should be blocked", cmd)
		}
	}
	for _, cmd := range []string{"ls -la", "go build ./...", "cat /etc/hosts"} {
		blocked := false
		for _, prefix := range dangerousPrefixes {
			if strings.HasPrefix(strings.TrimSpace(cmd), prefix) {
				blocked = true
				break
			}
		}
		if blocked {
			t.Errorf("command %q should NOT be blocked", cmd)
		}
	}
}

func TestReadFileOffsetLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = strings.Repeat("x", 10)
	}
	os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)

	rf := &ReadFile{}

	// basic read
	input, _ := json.Marshal(map[string]any{"path": path})
	result, err := rf.Execute(input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "xxxxxxxxxx") {
		t.Error("basic read failed")
	}

	// offset + limit
	input, _ = json.Marshal(map[string]any{"path": path, "offset": 5, "limit": 3})
	result, err = rf.Execute(input)
	if err != nil {
		t.Fatal(err)
	}
	// result should contain exactly 3 lines of content plus a "... (N more lines)" hint
	if !strings.Contains(result, "... (") {
		t.Error("expected truncation hint in output")
	}
	// count content lines (before the hint)
	contentLines := strings.Split(strings.Split(result, "\n...")[0], "\n")
	if len(contentLines) != 3 {
		t.Errorf("offset/limit read got %d content lines, want 3", len(contentLines))
	}
}
