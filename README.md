# lw

TUI for git worktrees with hook-based scriptability.

## Build

```bash
go build -o lw ./cmd/lw
```

Or install to `$GOPATH/bin`:

```bash
go install ./cmd/lw
```

## Usage

Run `lw` from inside any git repo with worktrees.

### Keys

| Key | Action |
|-----|--------|
| `j`/`k` | Navigate |
| `n` | Create worktree |
| `d` | Delete worktree |
| `o`/`Enter` | Open (fires `on_open` hook) |
| `p` | Prune stale worktrees |
| `v` | View details |
| `?` | Help |
| `Tab` | Switch panels |
| `r` | Refresh |
| `q` | Quit |

## Config

Place `lazywt.toml` in your repo root or `~/.config/lazywt/config.toml` globally.

```toml
[general]
shell = "bash -c"
default_path = "worktrees"

[hooks]
on_open = "cd $LW_PATH && $SHELL"
on_switch = ""
pre_create = ""
post_create = ""
pre_delete = ""
post_delete = ""

[display]
show_path = true
path_style = "relative"
```

### Hook env vars

All hooks receive: `LW_ACTION`, `LW_REPO_PATH`, `LW_PATH`, `LW_BRANCH`.
`on_open` and `on_switch` also get `LW_IS_DIRTY` (`0` or `1`).

Pre-hooks (`pre_create`, `pre_delete`, `pre_prune`) block the action on non-zero exit.
