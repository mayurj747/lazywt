# AGENTS.md — Coding Agent Guidelines

> Context for AI coding agents operating in this repository.

## Project Overview

- **Name**: lazywt (lazyworktree)
- **What**: A TUI for managing git worktrees with hook-based scriptability
- **Binary**: `lw`
- **Language**: Go 1.25
- **TUI framework**: Bubbletea (Charmbracelet ecosystem — lipgloss, bubbles)
- **Config format**: TOML (`lazywt.toml`), parsed with `github.com/BurntSushi/toml`
- **Package manager**: `go mod`
- **Monorepo**: No

## Build / Lint / Test Commands

```bash
# Build
go build -o lw ./cmd/lw

# Install to $GOPATH/bin
go install ./cmd/lw

# Lint / vet
go vet ./...

# Format (check + fix)
gofmt -w .

# Run ALL tests
go test ./...

# Run tests in a SINGLE package
go test ./internal/config
go test ./internal/git
go test ./internal/hooks

# Run a SINGLE test by name
go test ./internal/config -run TestLoadFromPaths_MergeBehavior
go test ./internal/git -run TestParsePorcelain
go test ./internal/hooks -run TestRun_EnvVarsPassedCorrectly

# Run tests with verbose output
go test -v ./internal/config -run TestLoadFile_FullConfig
```

## Architecture

```
cmd/
  lw/
    main.go              — Entry point: dispatches to runInit() or runTUI()
internal/
  config/
    config.go            — TOML config loading with layered merge (global + project)
    config_test.go       — Table-driven tests for config loading and merging
  git/
    loader.go            — Lists worktrees via `git worktree list --porcelain`
    loader_test.go       — Tests for porcelain parsing (pure, no git required)
    worktree.go          — Git worktree CRUD (create, delete, prune)
    status.go            — Git status queries (dirty check, last commit, tracking)
  hooks/
    executor.go          — Shell-based hook execution with env vars
    executor_test.go     — Tests for hook execution, exit codes, env passing
  model/
    worktree.go          — Worktree data model (shared across packages)
  tui/
    app.go               — Main Bubbletea model (App), state machine, key handling
    worktree_list.go     — Worktree list panel component
    command_pane.go      — Scrollable output pane for hook/git output
    keys.go              — Key constant definitions
    styles.go            — Lipgloss style definitions
```

## Code Style

### Formatting

- Use `gofmt` — standard Go formatting (tabs, no line-length limit enforced)
- Run `go vet ./...` after changes to catch issues

### Imports

Group imports in this order with a blank line between groups:
1. Standard library (`fmt`, `os`, `strings`, etc.)
2. Third-party packages (`github.com/charmbracelet/...`, `github.com/BurntSushi/toml`)
3. Internal packages (`github.com/mbency/lazyworktree/internal/...`)

Alias Bubbletea as `tea`:
```go
tea "github.com/charmbracelet/bubbletea"
```

### Naming Conventions

| Element          | Convention      | Example                          |
|------------------|-----------------|----------------------------------|
| Packages         | lowercase       | `config`, `git`, `hooks`, `tui`  |
| Files            | lowercase       | `loader.go`, `worktree.go`       |
| Exported funcs   | PascalCase      | `ListWorktrees`, `NewExecutor`   |
| Unexported funcs | camelCase       | `parsePorcelain`, `buildEnv`     |
| Types/structs    | PascalCase      | `Config`, `HookResult`, `App`    |
| Constants        | camelCase       | `keyJ`, `keyQ` (unexported)      |
| Exported consts  | PascalCase      | `DefaultShell`, `DefaultShowPath`|
| Enum-like consts | PascalCase+iota | `ListPanel`, `StateNormal`       |
| Test files       | `*_test.go`     | `config_test.go`                 |

### Error Handling

- Return `error` as the last return value; check it immediately
- Write errors to stderr: `fmt.Fprintf(os.Stderr, "Error: %v\n", err)`
- Exit with `os.Exit(1)` only at the top level (`main`)
- Use `errors.Is()` for sentinel comparisons (e.g., `os.ErrNotExist`)
- For non-fatal missing files, skip silently rather than failing
- Never swallow errors — propagate or log them

```go
// Pattern used throughout:
cfg, err := config.Load("lazywt.toml")
if err != nil {
    fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
    os.Exit(1)
}
```

### Testing

- Use Go's standard `testing` package — no external test frameworks
- **Table-driven tests** are the primary pattern — use `[]struct{...}` with `t.Run()`
- Test files are colocated: `config_test.go` next to `config.go`
- Use `t.Helper()` in test helpers
- Use `t.TempDir()` for filesystem tests (auto-cleaned)
- Use `t.Fatalf()` for setup failures, `t.Errorf()` for assertion failures
- Tests that parse git output are pure (no actual git repo needed)

```go
// Table-driven test pattern used in this project:
tests := []struct {
    name string
    got  string
    want string
}{
    {"PreCreate (overridden)", dst.Hooks.PreCreate, "src-pre-create"},
    {"OnOpen (overridden)", dst.Hooks.OnOpen, "src-on-open"},
}
for _, tt := range tests {
    if tt.got != tt.want {
        t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
    }
}
```

### Bubbletea Patterns

- The `App` struct is the root Bubbletea model implementing `Init()`, `Update()`, `View()`
- State machine via `AppState` enum (`StateNormal`, `StateCreating`, etc.)
- Key handling dispatched by state in `handleKey()` -> `handleNormalKey()`, etc.
- Async work via `tea.Cmd` functions that return `tea.Msg`
- Custom message types (e.g., `worktreesLoadedMsg`, `outputLineMsg`) for async results
- Panel focus tracked by `Panel` enum (`ListPanel`, `CmdPanel`)

### Comments & TODOs

- Doc comments on exported types and functions: `// FuncName does X.`
- Use `// TODO(username): description` format for pending work
- Inline comments explain *why*, not *what*

## Config System

Two-layer config merge: global (`~/.config/lazywt/config.toml`) + project (`lazywt.toml`).
Project values override global. Both are optional. Pointer fields (`*bool`, `*string`)
distinguish "not set" from zero values. Accessor methods (e.g., `ShellCmd()`) apply defaults.

## CLI Interface for Scripting Agents

`lw` exposes a non-TUI CLI that agents can call programmatically. All commands
must be run from the lazywt project root (the directory containing `lazywt.toml`
or the bare repo).

### List worktrees

```bash
# Human-readable
lw list

# Machine-readable JSON (use this in scripts/agents)
lw list --format=json
```

The JSON output is an array of `model.Worktree` objects:

```json
[
  {
    "Path": "/abs/path/to/worktrees/main",
    "Branch": "main",
    "Name": "main",
    "IsMain": false,
    "IsDirty": false,
    "IsIntegrated": false,
    "IsPathMissing": false,
    "LastCommitHash": "a1b2c3f",
    "LastCommitSubject": "fix: login bug",
    "LastCommitFullHash": "a1b2c3f...",
    "LastCommitAuthor": "Alice",
    "LastCommitDate": "2024-01-15T10:30:00Z",
    "TrackingBranch": "origin/main"
  }
]
```

### Create a worktree

```bash
lw create <branch>
```

- If `<branch>` exists locally, checks it out directly.
- If `<branch>` exists only on `origin`, creates a local tracking branch.
- If `<branch>` does not exist, creates a new branch from HEAD.
- Prints the worktree path on success (useful for `cd $(lw create feat-x)`).
- The directory name is sanitized: `/` and `\` in branch names are replaced with `-`.

### Delete a worktree

```bash
lw delete <branch>
```

Removes the worktree for the given branch. Retries with `--force` if a clean
removal fails. Prints the deleted path on success.

### Open a worktree (run on_open hook)

```bash
lw open <branch>
```

Runs the `on_open` hooks configured in `lazywt.toml` for the worktree on the
given branch. Prints the worktree path on success.

### Initialize a new project

```bash
lw init <git-url>       # non-interactive
lw init                 # interactive (prompts for URL and project name)
```

### Migrate an existing repo

```bash
lw migrate [path] [project-name]
```

Converts a standard cloned repo to the lazywt bare-repo layout. Creates a
sibling directory with the bare repo, `worktrees/`, `scripts/`, and `lazywt.toml`.

### Exit codes

All commands exit `0` on success and `1` on error. Error messages go to stderr.

### Example agent workflow

```bash
# List current worktrees as JSON
worktrees=$(lw list --format=json)

# Create a feature worktree and capture its path
wt_path=$(lw create feature/my-thing)

# Work in the worktree ...

# Delete when done
lw delete feature/my-thing
```

## Agent-Specific Rules

- **Always** run `go vet ./...` after editing Go files
- **Always** run related tests (`go test ./internal/<pkg>`) before considering a task done
- **Never** commit unless explicitly asked
- **Never** modify unrelated files while fixing a bug
- **Never** add new dependencies without confirming with the user
- **Prefer** editing existing files over creating new ones
- **Prefer** this project's existing patterns over external "best practices"
- When unsure about a convention, check 2-3 similar files in the codebase first
- Spec details are in `SPEC.md` — consult it for product requirements
