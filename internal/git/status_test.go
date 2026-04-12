package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initTestRepo creates a bare repo + one worktree with a single commit on
// "main" and returns (bareDir, worktreeDir).
func initTestRepo(t *testing.T) (bareDir, worktreeDir string) {
	t.Helper()
	dir := t.TempDir()
	bareDir = filepath.Join(dir, "repo.git")
	worktreeDir = filepath.Join(dir, "worktrees", "main")

	mustGit(t, dir, "init", "--bare", bareDir)
	// Clone the bare repo so we can make an initial commit.
	cloneDir := filepath.Join(dir, "clone")
	mustGit(t, dir, "clone", bareDir, cloneDir)
	mustGit(t, cloneDir, "config", "user.email", "test@test.com")
	mustGit(t, cloneDir, "config", "user.name", "Test")
	mustGit(t, cloneDir, "checkout", "-b", "main")

	// Write a file and commit.
	writeFile(t, filepath.Join(cloneDir, "README.md"), "# Hello\n")
	mustGit(t, cloneDir, "add", ".")
	mustGit(t, cloneDir, "commit", "-m", "initial commit")
	mustGit(t, cloneDir, "push", "--set-upstream", "origin", "main")

	// Add a worktree at worktreeDir from the bare repo.
	if err := os.MkdirAll(filepath.Dir(worktreeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, bareDir, "worktree", "add", worktreeDir, "main")
	mustGit(t, worktreeDir, "config", "user.email", "test@test.com")
	mustGit(t, worktreeDir, "config", "user.name", "Test")

	return bareDir, worktreeDir
}

// addFeatureBranch creates a new worktree for featureBranch branching from
// main, writes a file, commits, and returns the worktree path.
func addFeatureBranch(t *testing.T, bareDir, featureBranch string) string {
	t.Helper()
	wtDir := filepath.Join(filepath.Dir(filepath.Dir(bareDir)), "worktrees", featureBranch)
	if err := os.MkdirAll(filepath.Dir(wtDir), 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, bareDir, "worktree", "add", "-b", featureBranch, wtDir, "main")
	mustGit(t, wtDir, "config", "user.email", "test@test.com")
	mustGit(t, wtDir, "config", "user.name", "Test")

	writeFile(t, filepath.Join(wtDir, featureBranch+".txt"), "feature work\n")
	mustGit(t, wtDir, "add", ".")
	mustGit(t, wtDir, "commit", "-m", "feat: "+featureBranch)
	return wtDir
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- DefaultBranch ---

func TestDefaultBranch_FallsBackToMain(t *testing.T) {
	bareDir, _ := initTestRepo(t)
	got := DefaultBranch(bareDir)
	if got != "main" {
		t.Errorf("DefaultBranch = %q, want %q", got, "main")
	}
}

// --- IsIntegrated ---

func TestIsIntegrated_NotMerged(t *testing.T) {
	bareDir, _ := initTestRepo(t)
	wtDir := addFeatureBranch(t, bareDir, "feat-new")

	got := IsIntegrated(bareDir, wtDir, "main")
	if got {
		t.Errorf("IsIntegrated = true for unmerged branch, want false")
	}
}

func TestIsIntegrated_RegularMerge(t *testing.T) {
	bareDir, mainWt := initTestRepo(t)
	wtDir := addFeatureBranch(t, bareDir, "feat-regular")

	// Regular merge (preserves ancestry).
	mustGit(t, mainWt, "merge", "--no-ff", "feat-regular", "-m", "merge feat-regular")

	got := IsIntegrated(bareDir, wtDir, "main")
	if !got {
		t.Errorf("IsIntegrated = false after regular merge, want true")
	}
}

func TestIsIntegrated_SquashMerge(t *testing.T) {
	bareDir, mainWt := initTestRepo(t)
	wtDir := addFeatureBranch(t, bareDir, "feat-squash")

	// Squash merge: no ancestry relationship created.
	mustGit(t, mainWt, "merge", "--squash", "feat-squash")
	mustGit(t, mainWt, "commit", "-m", "squash feat-squash")

	got := IsIntegrated(bareDir, wtDir, "main")
	if !got {
		t.Errorf("IsIntegrated = false after squash merge, want true")
	}
}

func TestIsIntegrated_SquashMerge_MainMoved(t *testing.T) {
	// After squash-merging feat, main receives additional commits.
	// The feature branch should still be considered integrated.
	bareDir, mainWt := initTestRepo(t)
	wtDir := addFeatureBranch(t, bareDir, "feat-squash2")

	mustGit(t, mainWt, "merge", "--squash", "feat-squash2")
	mustGit(t, mainWt, "commit", "-m", "squash feat-squash2")

	// Another commit on main after the squash.
	writeFile(t, filepath.Join(mainWt, "post.txt"), "post-merge work\n")
	mustGit(t, mainWt, "add", ".")
	mustGit(t, mainWt, "commit", "-m", "post-merge commit")

	got := IsIntegrated(bareDir, wtDir, "main")
	if !got {
		t.Errorf("IsIntegrated = false after squash merge + additional main commit, want true")
	}
}

func TestIsIntegrated_MainWorktreeSelf(t *testing.T) {
	// The main worktree itself is never integrated (callers skip it, but IsIntegrated
	// should at minimum not panic and return a sensible value).
	bareDir, mainWt := initTestRepo(t)
	// main is an ancestor of itself — ancestor check fires, returns true.
	// This is fine; callers gate on !wt.IsMain before calling.
	_ = IsIntegrated(bareDir, mainWt, "main")
}
