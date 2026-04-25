package projectinit

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Migrate converts an existing standard git repo into a lazywt bare-repo layout.
// sourcePath is the path to the existing cloned repo. projectName is optional —
// if empty it is derived from the repo's remote URL or from the directory name.
//
// The resulting layout is created as a sibling directory of sourcePath:
//
//	<parent>/
//	  <projectName>/          ← new lazywt project root
//	    <projectName>.git/    ← bare clone of origin
//	    worktrees/
//	      <defaultBranch>/    ← worktree for the default branch
//	    scripts/
//	    lazywt.toml
//
// The original repo at sourcePath is not deleted; a message is printed
// instructing the user to remove it once they are happy with the migration.
func Migrate(sourcePath, projectName string) error {
	if sourcePath == "" {
		return fmt.Errorf("source path is required")
	}

	abs, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("resolving source path: %w", err)
	}

	// Ensure the source is a git repo.
	checkCmd := exec.Command("git", "-C", abs, "rev-parse", "--git-dir")
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("%q is not a git repository", abs)
	}

	// Obtain the remote URL (prefer "origin").
	remoteURL := remoteOriginURL(abs)

	if projectName == "" {
		if remoteURL != "" {
			projectName = ExtractProjectName(remoteURL)
		} else {
			projectName = filepath.Base(abs)
			projectName = strings.TrimSuffix(projectName, ".git")
		}
	}
	if projectName == "" {
		return fmt.Errorf("could not determine project name")
	}

	if remoteURL == "" {
		return fmt.Errorf("no remote URL found in %q; lazywt migration requires a remote origin to clone bare from", abs)
	}

	// Place the new project as a sibling of sourcePath.
	parentDir := filepath.Dir(abs)
	projectDir := filepath.Join(parentDir, projectName)

	if _, err := os.Stat(projectDir); err == nil {
		return fmt.Errorf("directory %q already exists", projectDir)
	}

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("creating project directory: %w", err)
	}

	// Clone bare repo from the remote.
	bareDir := filepath.Join(projectDir, projectName+".git")
	fmt.Printf("Cloning %s (bare) into %s ...\n", remoteURL, bareDir)
	if err := gitCloneBare(remoteURL, bareDir); err != nil {
		return fmt.Errorf("git clone --bare: %w", err)
	}

	// Detect default branch.
	branch, err := detectDefaultBranch(bareDir)
	if err != nil {
		return fmt.Errorf("detecting default branch: %w", err)
	}
	fmt.Printf("Default branch: %s\n", branch)

	// Create worktrees directory and check out default branch.
	worktreesDir := filepath.Join(projectDir, "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return fmt.Errorf("creating worktrees directory: %w", err)
	}

	wtPath := filepath.Join(worktreesDir, branch)
	fmt.Printf("Creating worktree: %s\n", wtPath)
	if err := gitWorktreeAdd(bareDir, wtPath, branch); err != nil {
		return fmt.Errorf("git worktree add: %w", err)
	}

	// Create scripts directory.
	scriptsDir := filepath.Join(projectDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return fmt.Errorf("creating scripts directory: %w", err)
	}

	// Scaffold lazywt.toml.
	configPath := filepath.Join(projectDir, "lazywt.toml")
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("writing lazywt.toml: %w", err)
	}

	fmt.Printf("Project %q migrated.\n", projectName)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  cd %s\n", projectDir)
	fmt.Printf("  lw\n")
	fmt.Printf("\nOnce you are satisfied, you can remove the original repo:\n")
	fmt.Printf("  rm -rf %s\n", abs)
	return nil
}

// remoteOriginURL returns the URL for the "origin" remote, or any remote if
// "origin" is not present. Returns empty string if no remotes are configured.
func remoteOriginURL(repoPath string) string {
	// Try origin first.
	out, err := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin").Output()
	if err == nil {
		if u := strings.TrimSpace(string(out)); u != "" {
			return u
		}
	}
	// Fall back to first remote.
	out, err = exec.Command("git", "-C", repoPath, "remote").Output()
	if err != nil {
		return ""
	}
	remotes := strings.Fields(string(out))
	if len(remotes) == 0 {
		return ""
	}
	out, err = exec.Command("git", "-C", repoPath, "remote", "get-url", remotes[0]).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

const defaultConfig = `# Hook environment variables:
#   $LW_ACTION    — action being performed (create, delete, open, switch, prune)
#   $LW_REPO_PATH — path to the bare repo / git dir
#   $LW_PATH      — path to the worktree (empty for prune)
#   $LW_BRANCH    — branch name (empty for prune)
#   $LW_IS_DIRTY  — "1" if worktree has uncommitted changes, "0" otherwise
#                    (only set for on_open)
#
# Pre-hooks block the action on non-zero exit.
# When a global config exists (~/.config/lazywt/config.toml), hooks chain
# by default (global runs first, then project). Override per-hook with:
#   [hooks.mode]
#   post_create = "override"  # "chain" (default), "override", or "disable"

[hooks]
pre_create = ""
post_create = ""
pre_delete = ""
post_delete = ""
on_open = ""
pre_prune = ""
post_prune = ""

[display]
show_path = true
path_style = "relative"

[general]
default_path = "worktrees"
shell = "sh -c"
`

// ExtractProjectName derives a project name from a git URL.
// Handles HTTPS, SSH, and local path formats.
func ExtractProjectName(url string) string {
	url = strings.TrimSpace(url)
	url = strings.TrimRight(url, "/")

	// SSH format: git@host:user/repo.git
	if idx := strings.LastIndex(url, ":"); idx != -1 && !strings.Contains(url, "://") {
		url = url[idx+1:]
	}

	name := filepath.Base(url)
	name = strings.TrimSuffix(name, ".git")
	return name
}

// Run performs the full project initialization:
//  1. Create project directory
//  2. Clone bare repo
//  3. Detect default branch
//  4. Create worktrees/ with default branch checked out
//  5. Create scripts/ directory
//  6. Scaffold lazywt.toml
func Run(url, projectName string) error {
	if url == "" {
		return fmt.Errorf("git URL is required")
	}
	if projectName == "" {
		projectName = ExtractProjectName(url)
	}
	if projectName == "" {
		return fmt.Errorf("could not determine project name from URL %q", url)
	}

	projectDir, err := filepath.Abs(projectName)
	if err != nil {
		return fmt.Errorf("resolving project path: %w", err)
	}

	// Fail if directory already exists
	if _, err := os.Stat(projectDir); err == nil {
		return fmt.Errorf("directory %q already exists", projectName)
	}

	// Create project directory
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("creating project directory: %w", err)
	}

	// Clone bare repo
	bareDir := filepath.Join(projectDir, projectName+".git")
	fmt.Printf("Cloning %s (bare) into %s ...\n", url, bareDir)
	if err := gitCloneBare(url, bareDir); err != nil {
		return fmt.Errorf("git clone --bare: %w", err)
	}

	// Detect default branch
	branch, err := detectDefaultBranch(bareDir)
	if err != nil {
		return fmt.Errorf("detecting default branch: %w", err)
	}
	fmt.Printf("Default branch: %s\n", branch)

	// Create worktrees directory and check out default branch
	worktreesDir := filepath.Join(projectDir, "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return fmt.Errorf("creating worktrees directory: %w", err)
	}

	wtPath := filepath.Join(worktreesDir, branch)
	fmt.Printf("Creating worktree: %s\n", wtPath)
	if err := gitWorktreeAdd(bareDir, wtPath, branch); err != nil {
		return fmt.Errorf("git worktree add: %w", err)
	}

	// Create scripts directory
	scriptsDir := filepath.Join(projectDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return fmt.Errorf("creating scripts directory: %w", err)
	}

	// Scaffold config
	configPath := filepath.Join(projectDir, "lazywt.toml")
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("writing lazywt.toml: %w", err)
	}

	fmt.Printf("Project %q initialized.\n", projectName)
	return nil
}

func gitCloneBare(url, dest string) error {
	cmd := exec.Command("git", "clone", "--bare", url, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func detectDefaultBranch(bareRepoPath string) (string, error) {
	cmd := exec.Command("git", "-C", bareRepoPath, "symbolic-ref", "--short", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	branch := strings.TrimSpace(out.String())
	if branch == "" {
		return "", fmt.Errorf("HEAD does not point to a branch")
	}
	return branch, nil
}

func gitWorktreeAdd(bareRepoPath, worktreePath, branch string) error {
	cmd := exec.Command("git", "-C", bareRepoPath, "worktree", "add", worktreePath, branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
