package projectinit

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultConfig = `[hooks]
pre_create = ""
post_create = ""
pre_delete = ""
post_delete = ""
on_switch = ""
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
