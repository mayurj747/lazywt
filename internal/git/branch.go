package git

import (
	"os/exec"
	"strings"
)

// ListBranches returns all local branch names for the given repo path.
func ListBranches(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoPath, "branch", "--format=%(refname:short)")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// ShowHead returns the full `git show` output for a given worktree or branch.
// If worktreePath is non-empty, it runs `git show HEAD` at that path.
// Otherwise it runs `git show <branch>` from repoPath.
func ShowHead(repoPath, worktreePath, branch string) (string, error) {
	var cmd *exec.Cmd
	if worktreePath != "" {
		cmd = exec.Command("git", "-C", worktreePath, "show", "HEAD", "--no-color")
	} else {
		cmd = exec.Command("git", "-C", repoPath, "show", branch, "--no-color")
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
