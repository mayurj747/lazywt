package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ResolveRepoPath determines the correct git repo path for lw to operate on.
// It handles three layouts:
//  1. cwd is a bare repo itself
//  2. cwd is inside a git worktree (regular or linked to a bare repo)
//  3. cwd is an lw project root containing a bare repo subdirectory
func ResolveRepoPath(cwd string) string {
	// Case 1: cwd is already a bare repo.
	if isBare, _ := IsBareRepo(cwd); isBare {
		return cwd
	}

	// Case 2: cwd is inside a git repo or worktree.
	// git --git-common-dir returns:
	//   - a relative path (e.g. ".git") when in the main worktree of a regular repo
	//   - an absolute path to the shared git dir when in a linked worktree
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--git-common-dir").Output()
	if err == nil {
		commonDir := strings.TrimSpace(string(out))
		if !filepath.IsAbs(commonDir) {
			commonDir = filepath.Clean(filepath.Join(cwd, commonDir))
		}
		// If the shared git dir is itself a bare repo, use it directly.
		if isBare, _ := IsBareRepo(commonDir); isBare {
			return commonDir
		}
		// Regular repo — run git commands from the working tree root.
		if root, err := RepoRoot(cwd); err == nil {
			return root
		}
		return cwd
	}

	// Case 3: cwd is an lw project root (not a git dir itself).
	// Scan subdirectories for a bare repo.
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return cwd
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(cwd, e.Name())
		if isBare, _ := IsBareRepo(candidate); isBare {
			return candidate
		}
	}

	return cwd
}

func Create(repoPath, worktreePath, branch, base string) error {
	args := []string{"-C", repoPath, "worktree", "add"}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, worktreePath)
	if base != "" {
		args = append(args, base)
	}

	cmd := exec.Command("git", args...)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func Delete(repoPath, worktreePath string, force bool) error {
	args := []string{"-C", repoPath, "worktree", "remove"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, worktreePath)

	cmd := exec.Command("git", args...)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func Prune(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "prune")
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func IsBareRepo(path string) (bool, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--is-bare-repository")
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return string(out) == "true\n", nil
}

func RepoRoot(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out[:len(out)-1]), nil
}
