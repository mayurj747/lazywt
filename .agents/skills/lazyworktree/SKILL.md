---
name: lazyworktree
version: 2
description: Safely manage git worktrees in a lazywt project using the lw CLI from anywhere inside the project — including from inside a worktree or the bare repo
license: MIT
compatibility: opencode
metadata:
  tool: lw
  workflow: worktree-management
---

## What I do

Teach agents how to manage git worktrees in a **lazywt (lazyworktree)** project
using the `lw` CLI, regardless of where the agent is currently running — inside a
worktree, inside the bare repo, or at the project root.

**Core rule:** `lw` commands must run from the **project root** (the directory
containing `lazywt.toml`). This skill shows you how to discover that root from
anywhere inside the project so your `lw` calls always do the right thing.

## When to use me

- Creating, deleting, listing, or opening worktrees via `lw` CLI
- Agent is invoked from inside a worktree or nested path
- Scripting lazywt operations from a subdomain, plugin, or IDE context that runs
  inside a worktree

---

## Project layout

```
<project-name>/                    # project root — WHERE lw MUST RUN
  <project-name>.git/              # bare git repo
  worktrees/                       # all linked worktrees
    main/                          # default-branch worktree
    feat-x/                        # one directory per branch/worktree
  scripts/                         # optional hook scripts
  lazywt.toml                      # config (hooks, display, general)
```

**Key point:** `lazywt.toml` lives at the project root. That file is the
anchor for finding the root from anywhere inside the project.

---

## Finding the project root safely

`lw` commands resolve the bare-repo path correctly even when run from inside a
worktree (see `ResolveRepoPath` in `internal/git/worktree.go`). **However**, the
`projectRoot` used to construct worktree paths (in `runCreate`, `resolvePaths`)
is simply `os.Getwd()`. This means if you call `lw create` from inside a
worktree, the new worktree gets nested inside the current one — wrong.

**Always cd to the project root before running `lw`.**

### Method 1: findup to `lazywt.toml` (preferred)

From any shell or script, find the project root by walking upward until
`lazywt.toml` is found:

```bash
# Find project root containing lazywt.toml
find_lw_root() {
  dir="$PWD"
  while [ "$dir" != "/" ]; do
    if [ -f "$dir/lazywt.toml" ]; then
      echo "$dir"
      return
    fi
    dir="$(dirname "$dir")"
  done
  echo "Error: could not find lazywt.toml" >&2
  return 1
}

# Store root once, use it for every lw call
LW_ROOT=$(find_lw_root)
cd "$LW_ROOT" || exit 1

# Now all lw commands are safe
lw list --format=json
lw create feature/my-thing
lw delete feature/my-thing
```

### Method 2: use `git rev-parse` + layout heuristics

If you are inside a worktree and `lazywt.toml` is not visible (e.g. it is
ignored or located differently), derive the root from the repo layout:

```bash
# Inside a worktree, find the bare repo's parent directory
BARE_REPO=$(git rev-parse --git-common-dir)
# If BARE_REPO is a relative path (e.g. ".git"), resolve it
[ "${BARE_REPO#/}" = "$BARE_REPO" ] && BARE_REPO="$(cd "$BARE_REPO" && pwd)"
# In lazywt layout, project root is the parent of the bare repo
LW_ROOT=$(dirname "$BARE_REPO")
cd "$LW_ROOT" || exit 1
```

### Method 3: prefix every `lw` call (if shell state must not change)

If changing `cd` globally is problematic, prefix each command:

```bash
LW_ROOT=$(find_lw_root)

project="$LW_ROOT" lw list --format=json
# ... or use subshells so state is isolated
(cd "$LW_ROOT" && lw create feature/my-thing)
```

**Choose Method 1 for readability, Method 3 for isolation.**

---

## CLI reference

All commands below assume you have first `cd`d to `$LW_ROOT`.

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
    "LastCommitFullHash": "a1b2c3f...",
    "LastCommitAuthor": "Alice",
    "LastCommitDate": "2024-01-15T10:30:00Z",
    "TrackingBranch": "origin/main"
  }
]
```

Key fields to check:
- `IsDirty` — uncommitted changes present
- `IsIntegrated` — branch work already merged into default branch (only true
  when **not** dirty)
- `IsPathMissing` — worktree directory no longer exists on disk (stale entry)

### Create a worktree

```bash
lw create <branch>
```

- Existing local branch → checked out directly
- Branch only on `origin` → local tracking branch created
- Branch not found anywhere → new branch created from HEAD
- Prints the worktree **absolute path** on success

Branch name sanitization: `/` and `\` are replaced with `-` in the directory
name, so `feature/my-thing` becomes `worktrees/feature-my-thing/`.

```bash
wt_path=$(lw create feature/my-thing)
echo "Created at: $wt_path"
```

### Delete a worktree

```bash
lw delete <branch>
```

- Removes the worktree for the given branch
- Retries with `--force` automatically if clean removal fails
- Prints the deleted path on success
- **Cannot delete the main worktree** (git-enforced restriction)

### Open a worktree (fire on_open hook)

```bash
lw open <branch>
```

Runs the `on_open` hooks from `lazywt.toml` for the matching worktree and
prints its path on success.

### Initialize a new project

```bash
lw init <git-url>       # non-interactive
lw init                 # interactive (prompts for URL and project name)
```

### Migrate an existing standard repo

```bash
lw migrate [path] [project-name]
```

Converts a regular cloned repo to the lazywt bare-repo layout, creating a
sibling directory with the bare repo, `worktrees/`, `scripts/`, and
`lazywt.toml`. The original repo is left intact.

---

## Exit codes

| Code | Meaning |
|------|---------|
| `0`  | Success |
| `1`  | Error (message written to stderr) |

---

## Typical safe agent workflow

```bash
#!/bin/bash
set -euo pipefail

# 1. Anchor ourselves at the project root
find_lw_root() {
  dir="$PWD"
  while [ "$dir" != "/" ]; do
    if [ -f "$dir/lazywt.toml" ]; then
      echo "$dir"
      return
    fi
    dir="$(dirname "$dir")"
  done
  echo "Error: could not find lazywt.toml" >&2
  return 1
}

LW_ROOT=$(find_lw_root)
cd "$LW_ROOT"

# 2. Inspect current state
worktrees=$(lw list --format=json)

# 3. Create a feature worktree
cd "$LW_ROOT"
wt_path=$(lw create feature/my-thing)
# wt_path is absolute, e.g. /abs/path/to/worktrees/feature-my-thing

# 4. Do work inside the worktree
cd "$wt_path"
# ... edit files, run tests, etc.

# 5. Return to root and delete when done
cd "$LW_ROOT"
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

- **Always anchor to the project root before `lw` calls.** Running `lw create`
  from inside a worktree will nest the new worktree inside the current one.
- `lazywt.toml` is the reliable anchor; if your environment may not have it
  visible, fall back to the `git rev-parse --git-common-dir` → `dirname`
  heuristic.
- The main worktree (the bare repo entry) is filtered from the TUI list and
  **cannot be deleted**.
- `IsIntegrated` is only `true` when the worktree is **not dirty**.
- Branch names with `/` or `\` are sanitized to `-` in directory names but the
  original branch name is preserved in git and all `lw` commands.
- `lw` does not auto-commit, auto-push, or auto-stash. If a worktree is dirty,
  `lw delete` with `--force` may still succeed, but consider warning the user
  about uncommitted work first.
