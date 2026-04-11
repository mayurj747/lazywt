# la-z-worktree

> A focused, scriptable TUI for managing git worktrees. Not a general git client.

**Product name**: lazyworktree (stylized: la-z-worktree)
**Binary**: `lw`
**Env prefix**: `LW_`
**Config file**: `lazywt.toml`

## Technical Stack
- **Language**: Go
- **TUI**: Bubbletea (Charmbracelet ecosystem — lipgloss, bubbles)
- **Config**: TOML
- **Testing**: Go standard library (`testing`), table-driven tests from day one

## Commands

### `lw` (no args)
Launch the TUI. Must be run from inside a lazywt-managed project directory (contains `lazywt.toml`) or a bare repo with worktrees.

### `lw init <url>`
Non-interactive project setup. Extracts project name from URL.

### `lw init`
Interactive mini-TUI prompting for:
1. Git URL (required)
2. Project name (optional — defaults to name extracted from URL)

Both modes produce the same layout:
```
acme/
├── acme.git/        # bare repo (git clone --bare)
├── worktrees/       # container for all worktrees
│   └── main/        # default branch worktree, created automatically
├── scripts/         # hook scripts (referenced from lazywt.toml)
└── lazywt.toml      # repo-scoped config (scaffolded with defaults)
```

## TUI

### Core UX
Two-panel layout: worktree list (top/main) + command pane (bottom). Keybinding-driven actions. No mouse support needed for v1.

### Worktree List — Each Row Shows:
```
● main    worktrees/main    a1b2c3f fix: login bug    [dirty]
  feat-x  worktrees/feat-x  d4e5f6a add: user profile
```
- Current worktree indicator (●)
- Branch name
- Relative path
- Last commit (short hash + subject)
- Dirty indicator (if working tree has uncommitted changes)

### Keybindings (v1 — hardcoded)
| Key | Action |
|-----|--------|
| `j`/`k` or `↑`/`↓` | Navigate list |
| `n` | Create new worktree |
| `d` | Delete worktree (with confirmation) |
| `enter` / `o` | Open worktree (fires `on_open` hook) |
| `p` | Prune stale worktrees |
| `v` | View worktree details (expanded info panel) |
| `r` | Refresh worktree list |
| `?` | Show help |
| `tab` | Toggle focus between worktree list and command pane |
| `C` | Clear command pane |
| `q` | Quit |

### View Details Panel
When `v` is pressed, show expanded info for selected worktree:
- Full branch name + tracking info (if any)
- Full path (absolute)
- HEAD commit (full hash + author + date + message)
- Clean/dirty status
- Whether it's the main worktree or a linked worktree

### Command Pane
Bottom panel that captures and displays hook output in real time.
 Shows stdout and stderr from hook executions as they run
 Each entry prefixed with the hook name and timestamp: `[on_open 10:32:05] Spawning terminal in /home/user/acme/worktrees/main`
 Scrollable — `tab` to focus the command pane, `j`/`k` to scroll, `tab` to return to worktree list
 Color-coded: stdout in default color, stderr in yellow, hook failures (non-zero exit) in red
 Cleared on `C` (clear log), persists across actions otherwise
 Also captures output from git commands run by lw itself (e.g. `git worktree add`, `git worktree remove`)

## Config

### Layered Loading
1. **Global**: `~/.config/lazywt/config.toml` — user-wide defaults
2. **Project**: `lazywt.toml` in project root — overrides global for this project

Project config wins on conflict. Both are optional.

### Config Shape
```toml
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
path_style = "relative"  # "relative" | "absolute"

[general]
default_path = "worktrees"  # where new worktrees are created within the project
shell = "sh -c"           # shell used to execute hooks (e.g. "bash -ic", "zsh -ic" for interactive shell with aliases)
```

## Hook System

### Design Philosophy
lazyworktree has **no built-in editor or shell integration**. The "Open" action fires the `on_open` hook — the user's script decides what happens (spawn a terminal tab, open an editor, create a tmux split, etc.). This keeps the TUI simple and pushes all behavior into user-controlled scripts.

### Hook Events
| Event | When | Can Block? |
|-------|------|------------|
| `pre_create` | Before `git worktree add` | Yes |
| `post_create` | After successful worktree creation | No |
| `pre_delete` | Before `git worktree remove` | Yes |
| `post_delete` | After successful worktree removal | No |
| `on_open` | When user presses `enter`/`o` on a worktree | No |
| `pre_prune` | Before `git worktree prune` | Yes |
| `post_prune` | After successful prune | No |

### Hook Execution
- Hooks are shell commands (single string), executed using the `shell` config option (default: `sh -c`)
- The hook string is passed as the final argument to the shell command
- **pre_ hooks**: Non-zero exit code aborts the action. Stderr is captured and shown as an error in the TUI.
- **post_ hooks / on_ hooks**: Fire-and-forget. Non-zero exit shows a warning but doesn't affect state.
- If hook is empty string or missing from config, it's a no-op.

### Environment Variables
Every hook receives context via environment variables:

| Variable | Description | Available In |
|----------|-------------|--------------|
| `LW_PATH` | Absolute path to the worktree | All hooks |
| `LW_BRANCH` | Branch name | All hooks |
| `LW_ACTION` | The action name (create, delete, open, prune) | All hooks |
| `LW_PROJECT` | Absolute path to the project root | All hooks |
| `LW_BARE_REPO` | Absolute path to the bare repo | All hooks |
| `LW_IS_DIRTY` | `"1"` if dirty, `"0"` if clean | on_open |

## Bare Repo Support
First-class. lazyworktree detects whether the repo is bare and adjusts path resolution accordingly. The `lw init` command creates bare repos by default — this is the intended workflow.

## Scope Boundaries
- **IN SCOPE**: worktree CRUD, worktree list/display, hooks, config, `lw init`, bare repo support, prune
- **OUT OF SCOPE (v1)**: staging, committing, branching, merging, rebasing, fetch/pull, lock/unlock, custom keybindings, non-interactive CLI mode (beyond init), mouse support
