# lw

TUI for managing git worktrees with hook-based scriptability.

## Install

```bash
go install ./cmd/lw
```

Or build locally:

```bash
go build -o lw ./cmd/lw
```

## Commands

### `lw`

Launch the TUI. Run from inside any git repository (bare or non-bare).

### `lw init <url>`

Clone a repo as a bare repository and scaffold a `lazywt.toml` config file. Can be run interactively (prompts for URL and project name) or non-interactively:

```bash
lw init https://github.com/user/repo.git
```

## Keybindings

### Global

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Cycle panel focus |
| `Ctrl+C` | Quit |

### Worktrees panel

| Key | Action |
|-----|--------|
| `j` / `k` / `↑` / `↓` | Navigate |
| `o` / `Enter` | Open worktree (fires `on_open` hook) |
| `n` | Create new worktree |
| `d` | Delete worktree |
| `p` | Prune stale worktrees |
| `v` | View worktree details |
| `s` | Toggle commit panel |
| `r` | Refresh |
| `/` | Filter list |
| `?` | Help |
| `q` | Quit |

### Branches panel

| Key | Action |
|-----|--------|
| `j` / `k` / `↑` / `↓` | Navigate |
| `o` / `Enter` | Open worktree if exists, otherwise create one |
| `s` | Toggle commit panel |
| `r` | Refresh |
| `/` | Filter list |
| `?` | Help |
| `q` | Quit |

### Commit panel

| Key | Action |
|-----|--------|
| `j` / `k` / `↑` / `↓` | Scroll |
| `?` | Help |
| `q` | Quit |

### Command output panel

| Key | Action |
|-----|--------|
| `j` / `k` / `↑` / `↓` | Scroll |
| `C` | Clear output |
| `?` | Help |
| `q` | Quit |

### Mouse

- **Click** a panel to focus it; click a row to select it.
- **Double-click** a worktree to open it; double-click a branch to open/create.
- **Scroll wheel** moves the cursor or scrolls content.

## Configuration

lw reads configuration from two locations:

1. **Global**: `~/.config/lazywt/config.toml` (or `$XDG_CONFIG_HOME/lazywt/config.toml`)
2. **Project**: `lazywt.toml` in the current working directory

Both files are optional. When both exist, they are merged — see [Hook chaining](#hook-chaining) below.

### Full reference

```toml
[general]
shell = "sh -c"            # shell used to execute hooks
default_path = "worktrees"  # directory under project root for new worktrees

[hooks]
pre_create  = ""  # runs before creating a worktree (blocks on failure)
post_create = ""  # runs after creating a worktree
pre_delete  = ""  # runs before deleting a worktree (blocks on failure)
post_delete = ""  # runs after deleting a worktree
on_open     = ""  # runs when opening a worktree (o / Enter)
pre_prune   = ""  # runs before pruning (blocks on failure)
post_prune  = ""  # runs after pruning

[display]
show_path = true       # show worktree paths in the list
path_style = "relative" # "relative" or "absolute"
```

### Hook environment variables

All hooks receive:

| Variable | Description |
|----------|-------------|
| `LW_ACTION` | The action being performed (`create`, `delete`, `open`, `prune`) |
| `LW_REPO_PATH` | Path to the bare repo / git dir |
| `LW_PATH` | Path to the worktree (empty for prune) |
| `LW_BRANCH` | Branch name (empty for prune) |

`on_open` also receives:

| Variable | Description |
|----------|-------------|
| `LW_IS_DIRTY` | `1` if the worktree has uncommitted changes, `0` otherwise |

### Pre-hook behavior

Pre-hooks (`pre_create`, `pre_delete`, `pre_prune`) run synchronously before the action. If any command in the chain exits non-zero, the action is aborted and remaining hooks are skipped.

### Post-hook behavior

Post-hooks (`post_create`, `post_delete`, `post_prune`) and event hooks (`on_open`, `on_switch`) run asynchronously with output streamed to the command output panel.

## Hook chaining

When both global and project configs define the same hook, lw **chains** them by default — the global hook runs first, then the project hook. This lets you set up system-wide behavior (e.g. terminal tab management) while projects add their own steps (e.g. dependency installation).

### Example

Global config (`~/.config/lazywt/config.toml`):
```toml
[hooks]
post_create = "python3 ~/.config/lazywt/scripts/wezterm-hooks.py post_create"
on_open     = "python3 ~/.config/lazywt/scripts/wezterm-hooks.py on_open"
```

Project config (`lazywt.toml`):
```toml
[hooks]
post_create = "cd $LW_PATH && npm install"
```

Result: when creating a worktree, the wezterm hook runs first, then `npm install`.

### Hook modes

Projects can override the default chaining behavior per-hook using `[hooks.mode]`:

```toml
[hooks]
post_create = "cd $LW_PATH && npm install"

[hooks.mode]
post_create = "override"  # only run the project hook, skip global
```

| Mode | Behavior |
|------|----------|
| `chain` | Run global hook, then project hook (default) |
| `override` | Run only the project hook, ignore global |
| `disable` | Run neither hook |

Modes are set per-hook. Hooks without a mode entry default to `chain`. An unrecognized mode value is treated as `chain`.

When no project config exists, or a hook is only defined in one config, it runs as a single command — functionally identical to the pre-chaining behavior.
