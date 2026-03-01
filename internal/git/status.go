package git

import (
	"bytes"
	"os/exec"
	"strings"
	"time"
)

// IsDirty checks if the worktree has uncommitted changes.
func IsDirty(worktreePath string) (bool, error) {
	cmd := exec.Command("git", "-C", worktreePath, "status", "--porcelain")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return false, err
	}
	return strings.TrimSpace(out.String()) != "", nil
}

// LastCommit returns the short hash and subject of the most recent commit.
func LastCommit(worktreePath string) (hash, subject string, err error) {
	cmd := exec.Command("git", "-C", worktreePath, "log", "-1", "--format=%h %s")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", "", err
	}
	parts := strings.SplitN(strings.TrimSpace(out.String()), " ", 2)
	if len(parts) < 2 {
		return parts[0], "", nil
	}
	return parts[0], parts[1], nil
}

// DetailedCommit returns full hash, author, date, and subject of the most recent commit.
// Uses a pipe delimiter (│) that's unlikely to appear in commit messages.
func DetailedCommit(worktreePath string) (fullHash, author, date, subject string, err error) {
	cmd := exec.Command("git", "-C", worktreePath, "log", "-1", "--format=%H│%an│%ai│%s")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", "", "", "", err
	}
	parts := strings.SplitN(strings.TrimSpace(out.String()), "│", 4)
	if len(parts) < 4 {
		return parts[0], "", "", "", nil
	}

	parsedDate, err := time.Parse("2006-01-02 15:04:05 -0700", parts[2])
	if err != nil {
		return parts[0], parts[1], parts[2], parts[3], nil
	}
	return parts[0], parts[1], parsedDate.Format(time.RFC3339), parts[3], nil
}

// TrackingBranch returns the upstream tracking branch for the current branch,
// or empty string if there's no upstream configured.
func TrackingBranch(worktreePath string) (string, error) {
	cmd := exec.Command("git", "-C", worktreePath, "rev-parse", "--abbrev-ref", "@{upstream}")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", nil
	}
	return strings.TrimSpace(out.String()), nil
}
