package git

import (
	"os/exec"
	"strings"
)

func IsRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	return cmd.Run() == nil
}

func HasChanges(dir string) bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

func AutoCommit(dir string, summary string) (string, error) {
	add := exec.Command("git", "add", "-A")
	add.Dir = dir
	if err := add.Run(); err != nil {
		return "", err
	}

	if len(summary) > 50 {
		summary = summary[:50]
	}
	commit := exec.Command("git", "commit", "-m", "axe: "+summary)
	commit.Dir = dir
	out, err := commit.Output()
	if err != nil {
		return "", err
	}

	// extract short hash
	rev := exec.Command("git", "rev-parse", "--short", "HEAD")
	rev.Dir = dir
	hash, err := rev.Output()
	if err != nil {
		// fallback: return raw output
		return strings.TrimSpace(string(out)), nil
	}
	return strings.TrimSpace(string(hash)), nil
}

func HasCommits(dir string) bool {
	cmd := exec.Command("git", "log", "-1", "--format=%s")
	cmd.Dir = dir
	out, err := cmd.Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

func Undo(dir string) (string, error) {
	// get last commit message for display
	msg := exec.Command("git", "log", "-1", "--format=%h %s")
	msg.Dir = dir
	out, _ := msg.Output()
	summary := strings.TrimSpace(string(out))

	cmd := exec.Command("git", "reset", "--mixed", "HEAD~1")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return summary, nil
}
