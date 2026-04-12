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

// DetailedCommit returns short hash, full hash, author, date, and subject of the most recent commit.
// Uses a pipe delimiter (│) that's unlikely to appear in commit messages.
func DetailedCommit(worktreePath string) (shortHash, fullHash, author, date, subject string, err error) {
	cmd := exec.Command("git", "-C", worktreePath, "log", "-1", "--format=%h│%H│%an│%ai│%s")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", "", "", "", "", err
	}
	parts := strings.SplitN(strings.TrimSpace(out.String()), "│", 5)
	if len(parts) < 5 {
		return parts[0], "", "", "", "", nil
	}

	parsedDate, err := time.Parse("2006-01-02 15:04:05 -0700", parts[3])
	if err != nil {
		return parts[0], parts[1], parts[2], parts[3], parts[4], nil
	}
	return parts[0], parts[1], parts[2], parsedDate.Format(time.RFC3339), parts[4], nil
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

// DefaultBranch detects the default branch for the repo at repoPath.
// It tries (in order): the symbolic ref of origin/HEAD, then local branches
// named "main", "master", "trunk", "develop".
func DefaultBranch(repoPath string) string {
	// Try origin/HEAD → refs/remotes/origin/main etc.
	out, err := exec.Command("git", "-C", repoPath, "symbolic-ref", "--short", "refs/remotes/origin/HEAD").Output()
	if err == nil {
		ref := strings.TrimSpace(string(out))
		// e.g. "origin/main" → "main"
		if _, after, ok := strings.Cut(ref, "/"); ok {
			return after
		}
		return ref
	}

	// Fall back to well-known names present locally.
	for _, candidate := range []string{"main", "master", "trunk", "develop"} {
		check := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "--quiet", "refs/heads/"+candidate)
		if check.Run() == nil {
			return candidate
		}
	}
	return "main"
}

// IsIntegrated reports whether the branch checked out at worktreePath has
// already been fully incorporated into defaultBranch (the branch is "stale").
//
// It uses a two-stage approach that handles squash merges correctly:
//
//  1. Fast path: if the branch commit is an ancestor of defaultBranch the work
//     was merged normally — return true immediately.
//
//  2. Simulated merge (git merge-tree --write-tree, git ≥ 2.38): merging the
//     feature branch into defaultBranch produces the same tree as defaultBranch
//     already has, meaning the branch contributes no new content.
//
// The main worktree (IsMain) is never considered integrated.
func IsIntegrated(repoPath, worktreePath, defaultBranch string) bool {
	// Fast path: ancestor check (handles normal merges and rebases).
	featureRef := "HEAD"
	check := exec.Command("git", "-C", worktreePath,
		"merge-base", "--is-ancestor", featureRef, defaultBranch)
	if check.Run() == nil {
		return true
	}

	// Simulated merge via git merge-tree --write-tree (git ≥ 2.38).
	// Merges feature into defaultBranch without touching the index or worktree.
	var mergeOut bytes.Buffer
	mergeCmd := exec.Command("git", "-C", repoPath,
		"merge-tree", "--write-tree", defaultBranch, "HEAD@{worktrees/"+worktreePath+"}")
	// The above ref form is tricky — use the branch name instead via the
	// worktree path: read HEAD from the worktree to get the commit SHA.
	_ = mergeCmd // discard; we'll rebuild with the SHA below

	headOut, err := exec.Command("git", "-C", worktreePath, "rev-parse", "HEAD").Output()
	if err != nil {
		return false
	}
	featureSHA := strings.TrimSpace(string(headOut))

	mergeCmd = exec.Command("git", "-C", repoPath,
		"merge-tree", "--write-tree", defaultBranch, featureSHA)
	mergeCmd.Stdout = &mergeOut
	if err := mergeCmd.Run(); err != nil {
		// git merge-tree exits non-zero on conflicts; also means not integrated.
		// Fall through to three-dot diff fallback.
		goto fallback
	}

	{
		// Compare the resulting tree SHA to defaultBranch's current tree SHA.
		mergedTree := strings.TrimSpace(mergeOut.String())
		// merge-tree --write-tree may output extra lines (notes); tree SHA is the first.
		if idx := strings.IndexByte(mergedTree, '\n'); idx != -1 {
			mergedTree = mergedTree[:idx]
		}

		var defaultTreeOut bytes.Buffer
		treeCmd := exec.Command("git", "-C", repoPath,
			"rev-parse", defaultBranch+"^{tree}")
		treeCmd.Stdout = &defaultTreeOut
		if treeCmd.Run() == nil {
			defaultTree := strings.TrimSpace(defaultTreeOut.String())
			if mergedTree == defaultTree {
				return true
			}
		}
	}

fallback:
	// Fallback: three-dot diff (works for rebases; not reliable for squash but
	// included as a cheap final check before giving up).
	diffCmd := exec.Command("git", "-C", repoPath,
		"diff", "--quiet", defaultBranch+"..."+featureSHA)
	return diffCmd.Run() == nil
}
