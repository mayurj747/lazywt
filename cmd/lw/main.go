package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mbency/lazyworktree/internal/config"
	"github.com/mbency/lazyworktree/internal/git"
	"github.com/mbency/lazyworktree/internal/hooks"
	projectinit "github.com/mbency/lazyworktree/internal/init"
	"github.com/mbency/lazyworktree/internal/model"
	"github.com/mbency/lazyworktree/internal/tui"
)

func main() {
	if len(os.Args) <= 1 {
		runTUI()
		return
	}

	switch os.Args[1] {
	case "init":
		runInit(os.Args[2:])
	case "migrate":
		runMigrate(os.Args[2:])
	case "list":
		runList(os.Args[2:])
	case "create":
		runCreate(os.Args[2:])
	case "delete":
		runDelete(os.Args[2:])
	case "open":
		runOpen(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command %q\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func runInit(args []string) {
	var url, name string

	if wantsHelp(args) {
		printInitUsage()
		return
	}
	if len(args) > 1 {
		fmt.Fprintln(os.Stderr, "Error: usage: lw init [url]")
		os.Exit(1)
	}

	if len(args) == 1 {
		// Non-interactive: lw init <url>
		url = args[0]
	} else {
		// Interactive: prompt for URL and optional project name
		scanner := bufio.NewScanner(os.Stdin)

		fmt.Print("Git URL: ")
		if scanner.Scan() {
			url = strings.TrimSpace(scanner.Text())
		}
		if url == "" {
			fmt.Fprintln(os.Stderr, "Error: git URL is required")
			os.Exit(1)
		}

		defaultName := projectinit.ExtractProjectName(url)
		fmt.Printf("Project name (%s): ", defaultName)
		if scanner.Scan() {
			name = strings.TrimSpace(scanner.Text())
		}
		// Empty input means accept the default
	}

	if err := projectinit.Run(url, name); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
func runMigrate(args []string) {
	if wantsHelp(args) {
		printMigrateUsage()
		return
	}
	if len(args) > 2 {
		fmt.Fprintln(os.Stderr, "Error: usage: lw migrate [path] [project-name]")
		os.Exit(1)
	}

	sourcePath := "."
	var projectName string
	if len(args) >= 1 {
		sourcePath = args[0]
	}
	if len(args) >= 2 {
		projectName = strings.TrimSpace(args[1])
	}

	if err := projectinit.Migrate(sourcePath, projectName); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runTUI() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load("lazywt.toml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	repoPath := git.ResolveRepoPath(cwd)
	app := tui.NewApp(cfg, repoPath, cwd)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runList(args []string) {
	if wantsHelp(args) {
		printListUsage()
		return
	}

	format := "text"
	for _, arg := range args {
		switch {
		case arg == "--format=json":
			format = "json"
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		default:
			fmt.Fprintf(os.Stderr, "Error: unknown list flag %q\n", arg)
			os.Exit(1)
		}
	}

	if format != "text" && format != "json" {
		fmt.Fprintf(os.Stderr, "Error: unsupported format %q (use text or json)\n", format)
		os.Exit(1)
	}

	cwd, repoPath, err := resolvePaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	worktrees, err := git.ListWorktrees(repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing worktrees: %v\n", err)
		os.Exit(1)
	}
	git.EnrichWorktreesConcurrent(worktrees, repoPath)

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(worktrees); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding json: %v\n", err)
			os.Exit(1)
		}
		return
	}

	for _, wt := range worktrees {
		rel, err := filepath.Rel(cwd, wt.Path)
		if err != nil {
			rel = wt.Path
		}
		if wt.Branch == "" {
			fmt.Printf("- (detached)\t%s\n", rel)
			continue
		}
		fmt.Printf("- %s\t%s\n", wt.Branch, rel)
	}
}

func runCreate(args []string) {
	if wantsHelp(args) {
		printCreateUsage()
		return
	}

	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Error: usage: lw create <branch>")
		os.Exit(1)
	}
	branch := strings.TrimSpace(args[0])
	if branch == "" {
		fmt.Fprintln(os.Stderr, "Error: branch is required")
		os.Exit(1)
	}

	projectRoot, repoPath, err := resolvePaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load("lazywt.toml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	wtPath := filepath.Join(projectRoot, cfg.DefaultPathDir(), branch)

	if git.BranchExists(repoPath, branch) {
		err = git.Create(repoPath, wtPath, "", branch)
	} else if remoteRef, ok := selectRemoteRef(repoPath, branch); ok {
		err = git.Create(repoPath, wtPath, branch, remoteRef)
	} else {
		err = git.Create(repoPath, wtPath, branch, "")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating worktree: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(wtPath)
}

func runDelete(args []string) {
	if wantsHelp(args) {
		printDeleteUsage()
		return
	}

	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Error: usage: lw delete <branch>")
		os.Exit(1)
	}
	branch := strings.TrimSpace(args[0])
	if branch == "" {
		fmt.Fprintln(os.Stderr, "Error: branch is required")
		os.Exit(1)
	}

	_, repoPath, err := resolvePaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	worktrees, err := git.ListWorktrees(repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing worktrees: %v\n", err)
		os.Exit(1)
	}

	wt := findWorktreeByBranch(worktrees, branch)
	if wt == nil {
		fmt.Fprintf(os.Stderr, "Error: no worktree found for branch %q\n", branch)
		os.Exit(1)
	}
	if wt.IsMain {
		fmt.Fprintln(os.Stderr, "Error: cannot delete main worktree")
		os.Exit(1)
	}

	err = git.Delete(repoPath, wt.Path, false)
	if err != nil {
		err = git.Delete(repoPath, wt.Path, true)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting worktree: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(wt.Path)
}

func runOpen(args []string) {
	if wantsHelp(args) {
		printOpenUsage()
		return
	}

	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Error: usage: lw open <branch>")
		os.Exit(1)
	}
	branch := strings.TrimSpace(args[0])
	if branch == "" {
		fmt.Fprintln(os.Stderr, "Error: branch is required")
		os.Exit(1)
	}

	projectRoot, repoPath, err := resolvePaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load("lazywt.toml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	worktrees, err := git.ListWorktrees(repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing worktrees: %v\n", err)
		os.Exit(1)
	}

	wt := findWorktreeByBranch(worktrees, branch)
	if wt == nil {
		fmt.Fprintf(os.Stderr, "Error: no worktree found for branch %q\n", branch)
		os.Exit(1)
	}

	env := map[string]string{
		"LW_ACTION":    "open",
		"LW_PROJECT":   projectRoot,
		"LW_BARE_REPO": repoPath,
		"LW_PATH":      wt.Path,
		"LW_BRANCH":    wt.Branch,
	}
	dirty, _ := git.IsDirty(wt.Path)
	if dirty {
		env["LW_IS_DIRTY"] = "1"
	} else {
		env["LW_IS_DIRTY"] = "0"
	}

	exec := hooks.NewExecutor(cfg.ShellCmd())
	for _, hookCmd := range cfg.Hooks.OnOpen {
		result := exec.Run(hookCmd, env)
		if result.Stdout != "" {
			fmt.Print(result.Stdout)
		}
		if result.Stderr != "" {
			fmt.Fprint(os.Stderr, result.Stderr)
		}
		if result.Err != nil {
			fmt.Fprintf(os.Stderr, "Error running on_open hook: %v\n", result.Err)
			os.Exit(1)
		}
		if result.ExitCode != 0 {
			os.Exit(result.ExitCode)
		}
	}

	fmt.Println(wt.Path)
}

func resolvePaths() (projectRoot, repoPath string, err error) {
	projectRoot, err = os.Getwd()
	if err != nil {
		return "", "", err
	}
	repoPath = git.ResolveRepoPath(projectRoot)
	if strings.TrimSpace(repoPath) == "" {
		return "", "", errors.New("could not resolve repository path")
	}
	return projectRoot, repoPath, nil
}

func selectRemoteRef(repoPath, branch string) (string, bool) {
	branches, err := git.ListBranches(repoPath)
	if err != nil {
		return "", false
	}
	return pickRemoteRef(branches, branch)
}

func pickRemoteRef(branches []git.Branch, branch string) (string, bool) {
	fallback := ""
	for _, b := range branches {
		if !b.IsRemote || b.Name != branch {
			continue
		}
		if strings.HasPrefix(b.Ref, "origin/") {
			return b.Ref, true
		}
		if fallback == "" {
			fallback = b.Ref
		}
	}
	if fallback != "" {
		return fallback, true
	}
	return "", false
}

func findWorktreeByBranch(worktrees []model.Worktree, branch string) *model.Worktree {
	for i := range worktrees {
		if worktrees[i].Branch == branch {
			return &worktrees[i]
		}
	}
	return nil
}

func printUsage() {
	fmt.Print(mainUsageText())
}

func printInitUsage() {
	fmt.Print(initUsageText())
}

func printListUsage() {
	fmt.Print(listUsageText())
}

func printCreateUsage() {
	fmt.Print(createUsageText())
}

func printDeleteUsage() {
	fmt.Print(deleteUsageText())
}

func printOpenUsage() {
	fmt.Print(openUsageText())
}

func printMigrateUsage() {
	fmt.Print(migrateUsageText())
}

func wantsHelp(args []string) bool {
	if len(args) != 1 {
		return false
	}
	switch args[0] {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func mainUsageText() string {
	return strings.Join([]string{
		"Usage:",
		"  lw                              Run TUI",
		"  lw init [url]                   Initialize a lazywt project",
		"  lw migrate [path] [name]        Migrate existing repo to lazywt layout",
		"  lw list [--format=json]         List worktrees",
		"  lw create <branch>              Create a worktree",
		"  lw delete <branch>              Delete a worktree by branch",
		"  lw open <branch>                Run on_open hook for branch worktree",
		"",
		"Help:",
		"  lw <command> --help",
		"",
	}, "\n")
}

func initUsageText() string {
	return strings.Join([]string{
		"Usage:",
		"  lw init [url]",
		"",
		"Description:",
		"  Initialize a lazywt project. If url is omitted, prompts interactively.",
		"",
		"Examples:",
		"  lw init git@github.com:owner/repo.git",
		"  lw init",
		"",
	}, "\n")
}

func listUsageText() string {
	return strings.Join([]string{
		"Usage:",
		"  lw list [--format=json]",
		"",
		"Description:",
		"  List worktrees for scripting or quick inspection.",
		"",
		"Options:",
		"  --format=json   Emit machine-readable JSON",
		"",
		"Examples:",
		"  lw list",
		"  lw list --format=json",
		"",
	}, "\n")
}

func createUsageText() string {
	return strings.Join([]string{
		"Usage:",
		"  lw create <branch>",
		"",
		"Description:",
		"  Create a worktree for branch.",
		"  If branch exists locally, it is used directly.",
		"  If branch exists only remotely, a local tracking branch is created.",
		"",
		"Example:",
		"  lw create feat-x",
		"",
	}, "\n")
}

func deleteUsageText() string {
	return strings.Join([]string{
		"Usage:",
		"  lw delete <branch>",
		"",
		"Description:",
		"  Delete the worktree currently checked out on branch.",
		"",
		"Example:",
		"  lw delete feat-x",
		"",
	}, "\n")
}

func openUsageText() string {
	return strings.Join([]string{
		"Usage:",
		"  lw open <branch>",
		"",
		"Description:",
		"  Run on_open hooks for the worktree on branch.",
		"",
		"Example:",
		"  lw open feat-x",
		"",
	}, "\n")
}

func migrateUsageText() string {
	return strings.Join([]string{
		"Usage:",
		"  lw migrate [path] [project-name]",
		"",
		"Description:",
		"  Migrate an existing standard git repo to the lazywt bare-repo layout.",
		"  path defaults to the current directory.",
		"  project-name defaults to the name derived from the remote URL.",
		"",
		"  The new lazywt project is created as a sibling of path:",
		"    <parent>/",
		"      <project-name>/          lazywt project root",
		"        <project-name>.git/    bare clone",
		"        worktrees/",
		"          <default-branch>/",
		"        scripts/",
		"        lazywt.toml",
		"",
		"  The original repo is NOT deleted; instructions are printed at the end.",
		"",
		"Examples:",
		"  lw migrate                   migrate repo in current directory",
		"  lw migrate ~/repos/myrepo    migrate a specific repo",
		"  lw migrate . myproject       migrate with an explicit project name",
		"",
	}, "\n")
}
