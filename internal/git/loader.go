package git

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mbency/lazyworktree/internal/model"
)

func ListWorktrees(repoPath string) ([]model.Worktree, error) {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	worktrees, err := parsePorcelain(out.String())
	if err != nil {
		return nil, err
	}

	for i := range worktrees {
		if i == 0 {
			worktrees[i].IsMain = true
		}
		worktrees[i].Name = deriveName(worktrees[i].Path, repoPath)
	}

	return worktrees, nil
}

func parsePorcelain(output string) ([]model.Worktree, error) {
	output = strings.ReplaceAll(output, "\r\n", "\n")
	stanzas := strings.Split(strings.TrimSpace(output), "\n\n")

	var worktrees []model.Worktree
	for _, stanza := range stanzas {
		if stanza == "" {
			continue
		}
		wt, isBare, err := parseStanza(stanza)
		if err != nil {
			return nil, err
		}
		// Skip the bare repo entry — it's not a real worktree.
		if isBare {
			continue
		}
		worktrees = append(worktrees, wt)
	}

	return worktrees, nil
}

func parseStanza(stanza string) (model.Worktree, bool, error) {
	wt := model.Worktree{}
	isBare := false
	lines := strings.Split(stanza, "\n")

	for _, line := range lines {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 0 {
			continue
		}

		key := parts[0]
		var value string
		if len(parts) == 2 {
			value = parts[1]
		}

		switch key {
		case "worktree":
			wt.Path = value
		case "HEAD":
			wt.LastCommitHash = value
		case "branch":
			wt.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "bare":
			isBare = true
		}
	}

	if _, err := os.Stat(wt.Path); errors.Is(err, os.ErrNotExist) {
		wt.IsPathMissing = true
	}

	return wt, isBare, nil
}

func deriveName(path, repoPath string) string {
	leaf := filepath.Base(path)
	if leaf == "." || leaf == "" {
		absPath, _ := filepath.Abs(path)
		return absPath
	}
	return leaf
}

// EnrichWorktreesConcurrent runs all enrichment queries for each worktree in
// parallel. repoPath is the bare repo / git common dir used for repo-level
// queries (default branch detection, merge-tree).
func EnrichWorktreesConcurrent(worktrees []model.Worktree, repoPath string) {
	// Detect the default branch once; used for the integration check.
	defaultBranch := DefaultBranch(repoPath)

	var wg sync.WaitGroup

	for i := range worktrees {
		if worktrees[i].IsPathMissing {
			continue
		}
		wg.Add(1)
		go func(wt *model.Worktree) {
			defer wg.Done()

			dirty, _ := IsDirty(wt.Path)
			wt.IsDirty = dirty

			// DetailedCommit returns both short and full hash; LastCommit is not needed.
			shortHash, fullHash, author, date, sub, _ := DetailedCommit(wt.Path)
			wt.LastCommitHash = shortHash
			wt.LastCommitFullHash = fullHash
			wt.LastCommitAuthor = author
			wt.LastCommitDate, _ = time.Parse(time.RFC3339, date)
			wt.LastCommitSubject = sub

			tracking, _ := TrackingBranch(wt.Path)
			wt.TrackingBranch = tracking

			// Integration check: is the branch's work already in the default branch?
			// Skip the main worktree (it IS the default branch) and detached HEADs.
			if !wt.IsMain && wt.Branch != "" && wt.Branch != defaultBranch {
				wt.IsIntegrated = IsIntegrated(repoPath, wt.Path, defaultBranch)
			}
		}(&worktrees[i])
	}

	wg.Wait()
}
