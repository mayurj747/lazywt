package git

import (
	"os/exec"
	"strings"
)

// Branch describes a branch entry shown in the branch panel.
//
// Name is the local branch name (without remote prefix), Ref is the git
// reference used for show/create actions, and Display is the list label.
type Branch struct {
	Name     string
	Ref      string
	Display  string
	IsRemote bool
}

// ListBranches returns local and remote branches for the given repo path.
func ListBranches(repoPath string) ([]Branch, error) {
	localOut, err := exec.Command("git", "-C", repoPath, "branch", "--format=%(refname:short)").Output()
	if err != nil {
		return nil, err
	}

	remoteOut, err := exec.Command("git", "-C", repoPath, "branch", "-r", "--format=%(refname:short)").Output()
	if err != nil {
		return nil, err
	}

	branches := parseBranches(string(localOut), false)
	branches = append(branches, parseBranches(string(remoteOut), true)...)
	return branches, nil
}

func parseBranches(raw string, isRemote bool) []Branch {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	branches := make([]Branch, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if isRemote {
			if strings.HasSuffix(line, "/HEAD") {
				continue
			}
			name := line
			if _, after, ok := strings.Cut(line, "/"); ok {
				name = after
			}
			branches = append(branches, Branch{
				Name:     name,
				Ref:      line,
				Display:  line,
				IsRemote: true,
			})
			continue
		}

		branches = append(branches, Branch{
			Name:     line,
			Ref:      line,
			Display:  line,
			IsRemote: false,
		})
	}
	return branches
}

// BranchExists checks whether a local branch with the given name exists.
func BranchExists(repoPath, branch string) bool {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "refs/heads/"+branch)
	return cmd.Run() == nil
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
