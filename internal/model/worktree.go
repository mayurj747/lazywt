package model

import "time"

// Worktree represents a single git worktree with its metadata.
type Worktree struct {
	// Core fields (from git worktree list --porcelain)
	Path          string // Absolute path to the worktree
	Branch        string // Branch checked out (empty for detached HEAD)
	Name          string // Display name, derived from path (uniquified leaf)
	GitDir        string // Path to the .git directory for this worktree
	IsMain        bool // True if this is the main worktree (first in porcelain output)
	IsPathMissing bool // True if the worktree path no longer exists on disk

	// Enrichment fields (from git log, git status)
	LastCommitHash    string // Short commit hash (e.g. "a1b2c3f")
	LastCommitSubject string // First line of commit message
	IsDirty           bool   // True if working tree has uncommitted changes

	// Detail fields (for the details panel)
	LastCommitFullHash string    // Full 40-char commit hash
	LastCommitAuthor   string    // Commit author name
	LastCommitDate     time.Time // Commit author date
	TrackingBranch     string    // Upstream tracking branch (empty if none)
}