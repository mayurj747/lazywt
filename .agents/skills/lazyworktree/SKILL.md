---
name: lazyworktree
description: Manage git worktrees in a lazywt (lazyworktree) project using the lw CLI — create, list, delete, and open worktrees programmatically
license: MIT
compatibility: opencode
metadata:
  tool: lw
  workflow: worktree-management
---

## What I do

Teach agents how to manage git worktrees in a **lazyworktree** project using
the `lw` CLI. This covers listing, creating, deleting, and opening worktrees
without touching the TUI.

## When to use me

Use this skill whenever you are working inside a lazywt-managed project and
need to create a worktree for a branch, clean up merged worktrees, or inspect
what worktrees currently exist.

---

## Project layout

A lazywt project looks like this:

```
<project-name>/
  <project-name>.git/   # bare git repo
  worktrees/            # all linked worktrees live here
    main/               # default-branch worktree
    feat-x/             # one directory per worktree
  scripts/              # optional hook scripts
  lazywt.toml           # project config (hooks, display, general settings)
```

All `lw` commands must be run from the **project root** — the directory that
contains `lazywt.toml` (or the bare repo, if there is no config file).

---

## CLI reference

### List worktrees

```bash
# Human-readable table
lw list

# Machine-readable JSON — use this in scripts
lw list --format=json
```

JSON shape (`[]model.Worktree`):

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
    "LastCommitFullHash": "a1b2c3fabcdef...",
    "LastCommitAuthor": "Alice",
    "LastCommitDate": "2024-01-15T10:30:00Z",
    "TrackingBranch": "origin/main"
  }
]
```

Key fields to check:
- `IsDirty` — worktree has uncommitted changes
- `IsIntegrated` — branch work is already merged into the default branch
- `IsPathMissing` — worktree directory no longer exists on disk (stale)

### Create a worktree

```bash
lw create <branch>
```

- Existing local branch → checked out directly
- Branch only on `origin` → local tracking branch is created
- Branch not found anywhere → new branch from HEAD
- Prints the worktree **path** on success

Branch name sanitization: `/` and `\` are replaced with `-` in the directory
name, so `feature/my-thing` lands at `worktrees/feature-my-thing`.

```bash
# Capture the path for later use
wt_path=$(lw create feature/my-thing)
echo "Worktree is at: $wt_path"
```

### Delete a worktree

```bash
lw delete <branch>
```

- Removes the worktree for the given branch
- Retries with `--force` automatically if a clean removal fails
- Prints the deleted path on success
- The **main worktree cannot be deleted** (git hard-codes this restriction)

### Open a worktree (fire on_open hook)

```bash
lw open <branch>
```

Runs the `on_open` hooks from `lazywt.toml` for the matching worktree and
prints its path on success.

### Initialize a new project

```bash
lw init <git-url>   # non-interactive
lw init             # interactive (prompts for URL and optional project name)
```

### Migrate an existing standard repo

```bash
lw migrate [path] [project-name]
```

Converts a regular cloned repo to the lazywt bare-repo layout. Creates a
sibling directory with the bare repo, `worktrees/`, `scripts/`, and
`lazywt.toml`. The original repo is left intact.

---

## Exit codes

| Code | Meaning |
|------|---------|
| `0`  | Success |
| `1`  | Error (message written to stderr) |

---

## Typical agent workflow

```bash
# 1. Check what worktrees currently exist
lw list --format=json

# 2. Create a feature worktree
wt_path=$(lw create feature/my-thing)
# wt_path is now /abs/path/to/worktrees/feature-my-thing

# 3. Do work inside the worktree ...

# 4. Delete when done (after merging)
lw delete feature/my-thing
```

---

## Config (lazywt.toml)

```toml
[hooks]
pre_create  = ""   # runs before git worktree add; non-zero exit aborts
post_create = ""   # runs after successful creation
pre_delete  = ""   # runs before git worktree remove; non-zero exit aborts
post_delete = ""   # runs after successful deletion
on_open     = ""   # fired by lw open / TUI 'o' key
pre_prune   = ""   # runs before git worktree prune
post_prune  = ""   # runs after successful prune

[display]
show_path  = true        # show path in TUI list
path_style = "relative"  # "relative" | "absolute"

[general]
default_path = "worktrees"  # where new worktrees are created
shell        = "sh -c"      # shell used to execute hooks
```

Hook environment variables available in every hook:

| Variable      | Description                                    |
|---------------|------------------------------------------------|
| `LW_PATH`     | Absolute path to the worktree                  |
| `LW_BRANCH`   | Branch name                                    |
| `LW_ACTION`   | Action name: create, delete, open, prune       |
| `LW_PROJECT`  | Absolute path to the project root              |
| `LW_BARE_REPO`| Absolute path to the bare repo                 |
| `LW_IS_DIRTY` | `"1"` if dirty, `"0"` if clean (on_open only)  |

---

## Notes and gotchas

- Always run `lw` from the **project root** (directory containing `lazywt.toml`)
- The main worktree (the bare repo entry) is filtered from the TUI list and
  **cannot be deleted** — all user-visible worktrees are linked and freely removable
- `IsIntegrated` is only `true` when the worktree is **not dirty** — dirty
  worktrees are never considered merged even if HEAD is an ancestor
- Branch names with `/` or `\` are sanitized to `-` in the directory name but
  the original branch name is preserved in git and in all `lw` commands
