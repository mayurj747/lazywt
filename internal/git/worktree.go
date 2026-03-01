package git

import (
	"os/exec"
)

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

func BareRepoGitDir(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--git-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out[:len(out)-1]), nil
}
