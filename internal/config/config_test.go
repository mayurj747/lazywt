package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// helper to write a temp TOML file, returns its path.
func writeTOML(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// boolPtr returns a pointer to a bool value.
func boolPtr(v bool) *bool { return &v }

// strPtr returns a pointer to a string value.
func strPtr(v string) *string { return &v }

func TestLoadFile_FullConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeTOML(t, dir, "config.toml", `
[hooks]
pre_create = "echo creating"
post_create = "echo created"
on_open = "./scripts/open.sh"

[display]
show_path = false
path_style = "absolute"

[general]
default_path = "wt"
shell = "bash -c"
`)

	cfg, err := LoadFromPaths(path, "")
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}

	// Hooks — now []string
	assertSlice(t, "PreCreate", cfg.Hooks.PreCreate, []string{"echo creating"})
	assertSlice(t, "PostCreate", cfg.Hooks.PostCreate, []string{"echo created"})
	assertSlice(t, "OnOpen", cfg.Hooks.OnOpen, []string{"./scripts/open.sh"})
	// Unset hooks should be nil
	assertSlice(t, "PreDelete", cfg.Hooks.PreDelete, nil)

	// Display
	if cfg.ShowPath() != false {
		t.Errorf("ShowPath() = %v, want false", cfg.ShowPath())
	}
	if cfg.PathStyle() != "absolute" {
		t.Errorf("PathStyle() = %q, want %q", cfg.PathStyle(), "absolute")
	}

	// General
	if cfg.DefaultPathDir() != "wt" {
		t.Errorf("DefaultPathDir() = %q, want %q", cfg.DefaultPathDir(), "wt")
	}
	if cfg.ShellCmd() != "bash -c" {
		t.Errorf("ShellCmd() = %q, want %q", cfg.ShellCmd(), "bash -c")
	}
}

func TestDefaults_EmptyConfig(t *testing.T) {
	cfg := &Config{}

	if cfg.ShowPath() != DefaultShowPath {
		t.Errorf("ShowPath() = %v, want %v", cfg.ShowPath(), DefaultShowPath)
	}
	if cfg.PathStyle() != DefaultPathStyle {
		t.Errorf("PathStyle() = %q, want %q", cfg.PathStyle(), DefaultPathStyle)
	}
	if cfg.DefaultPathDir() != DefaultDefPath {
		t.Errorf("DefaultPathDir() = %q, want %q", cfg.DefaultPathDir(), DefaultDefPath)
	}
	if cfg.ShellCmd() != DefaultShell {
		t.Errorf("ShellCmd() = %q, want %q", cfg.ShellCmd(), DefaultShell)
	}
}

func TestLoadFromPaths_MissingFiles(t *testing.T) {
	cfg, err := LoadFromPaths("/nonexistent/global.toml", "/nonexistent/project.toml")
	if err != nil {
		t.Fatalf("expected no error for missing files, got: %v", err)
	}
	if cfg.ShellCmd() != DefaultShell {
		t.Errorf("ShellCmd() = %q, want default %q", cfg.ShellCmd(), DefaultShell)
	}
}

func TestLoadFromPaths_EmptyPaths(t *testing.T) {
	cfg, err := LoadFromPaths("", "")
	if err != nil {
		t.Fatalf("expected no error for empty paths, got: %v", err)
	}
	if cfg.ShowPath() != DefaultShowPath {
		t.Errorf("ShowPath() = %v, want default %v", cfg.ShowPath(), DefaultShowPath)
	}
}

func TestLoadFromPaths_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := writeTOML(t, dir, "bad.toml", `this is not valid toml = = =`)

	_, err := LoadFromPaths(path, "")
	if err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}
}

func TestLoadFromPaths_GlobalOnly(t *testing.T) {
	dir := t.TempDir()
	globalPath := writeTOML(t, dir, "global.toml", `
[hooks]
on_open = "global-open"

[general]
shell = "zsh -c"
`)

	cfg, err := LoadFromPaths(globalPath, "")
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}
	assertSlice(t, "OnOpen", cfg.Hooks.OnOpen, []string{"global-open"})
	if cfg.ShellCmd() != "zsh -c" {
		t.Errorf("ShellCmd() = %q, want %q", cfg.ShellCmd(), "zsh -c")
	}
}

func TestLoadFromPaths_ProjectOnly(t *testing.T) {
	dir := t.TempDir()
	projPath := writeTOML(t, dir, "lazywt.toml", `
[hooks]
on_open = "project-open"

[display]
show_path = false
`)

	cfg, err := LoadFromPaths("", projPath)
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}
	assertSlice(t, "OnOpen", cfg.Hooks.OnOpen, []string{"project-open"})
	if cfg.ShowPath() != false {
		t.Errorf("ShowPath() = %v, want false", cfg.ShowPath())
	}
	if cfg.ShellCmd() != DefaultShell {
		t.Errorf("ShellCmd() = %q, want default %q", cfg.ShellCmd(), DefaultShell)
	}
}

func TestLoadFromPaths_ChainDefault(t *testing.T) {
	dir := t.TempDir()

	globalPath := writeTOML(t, dir, "global.toml", `
[hooks]
post_create = "global-post-create"
on_open = "global-open"

[display]
show_path = true
path_style = "relative"

[general]
default_path = "global-wt"
shell = "bash -c"
`)

	projPath := writeTOML(t, dir, "project.toml", `
[hooks]
post_create = "project-post-create"
on_open = "project-open"

[display]
show_path = false

[general]
default_path = "worktrees"
`)

	cfg, err := LoadFromPaths(globalPath, projPath)
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}

	// Chain: both global and project run
	assertSlice(t, "PostCreate", cfg.Hooks.PostCreate, []string{"global-post-create", "project-post-create"})
	assertSlice(t, "OnOpen", cfg.Hooks.OnOpen, []string{"global-open", "project-open"})

	// Display/General merge unchanged
	if cfg.ShowPath() != false {
		t.Errorf("ShowPath() = %v, want false (project override)", cfg.ShowPath())
	}
	if cfg.DefaultPathDir() != "worktrees" {
		t.Errorf("DefaultPathDir() = %q, want %q", cfg.DefaultPathDir(), "worktrees")
	}
	if cfg.ShellCmd() != "bash -c" {
		t.Errorf("ShellCmd() = %q, want %q", cfg.ShellCmd(), "bash -c")
	}
	if cfg.PathStyle() != "relative" {
		t.Errorf("PathStyle() = %q, want %q", cfg.PathStyle(), "relative")
	}
}

func TestLoadFromPaths_OverrideMode(t *testing.T) {
	dir := t.TempDir()

	globalPath := writeTOML(t, dir, "global.toml", `
[hooks]
post_create = "global-post-create"
on_open = "global-open"
`)

	projPath := writeTOML(t, dir, "project.toml", `
[hooks]
post_create = "project-post-create"

[hooks.mode]
post_create = "override"
`)

	cfg, err := LoadFromPaths(globalPath, projPath)
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}

	// Override: only project runs
	assertSlice(t, "PostCreate", cfg.Hooks.PostCreate, []string{"project-post-create"})
	// on_open not overridden — chains (but project didn't set it, so global only)
	assertSlice(t, "OnOpen", cfg.Hooks.OnOpen, []string{"global-open"})
}

func TestLoadFromPaths_DisableMode(t *testing.T) {
	dir := t.TempDir()

	globalPath := writeTOML(t, dir, "global.toml", `
[hooks]
post_create = "global-post-create"
on_open = "global-open"
`)

	projPath := writeTOML(t, dir, "project.toml", `
[hooks.mode]
post_create = "disable"
`)

	cfg, err := LoadFromPaths(globalPath, projPath)
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}

	// Disable: neither runs
	assertSlice(t, "PostCreate", cfg.Hooks.PostCreate, nil)
	// on_open not disabled — global preserved
	assertSlice(t, "OnOpen", cfg.Hooks.OnOpen, []string{"global-open"})
}

func TestLoadFromPaths_MixedModes(t *testing.T) {
	dir := t.TempDir()

	globalPath := writeTOML(t, dir, "global.toml", `
[hooks]
post_create = "global-post-create"
on_open = "global-open"
pre_delete = "global-pre-delete"
`)

	projPath := writeTOML(t, dir, "project.toml", `
[hooks]
post_create = "project-post-create"
on_open = "project-open"

[hooks.mode]
post_create = "chain"
on_open = "override"
pre_delete = "disable"
`)

	cfg, err := LoadFromPaths(globalPath, projPath)
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}

	assertSlice(t, "PostCreate (chain)", cfg.Hooks.PostCreate, []string{"global-post-create", "project-post-create"})
	assertSlice(t, "OnOpen (override)", cfg.Hooks.OnOpen, []string{"project-open"})
	assertSlice(t, "PreDelete (disable)", cfg.Hooks.PreDelete, nil)
}

func TestLoadFromPaths_InvalidModeFallsBackToChain(t *testing.T) {
	dir := t.TempDir()

	globalPath := writeTOML(t, dir, "global.toml", `
[hooks]
post_create = "global-post-create"
`)

	projPath := writeTOML(t, dir, "project.toml", `
[hooks]
post_create = "project-post-create"

[hooks.mode]
post_create = "bogus"
`)

	cfg, err := LoadFromPaths(globalPath, projPath)
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}

	// Invalid mode falls back to chain
	assertSlice(t, "PostCreate", cfg.Hooks.PostCreate, []string{"global-post-create", "project-post-create"})
}

func TestLoadFromPaths_OverrideWithEmptyProjectHook(t *testing.T) {
	dir := t.TempDir()

	globalPath := writeTOML(t, dir, "global.toml", `
[hooks]
post_create = "global-post-create"
`)

	projPath := writeTOML(t, dir, "project.toml", `
[hooks.mode]
post_create = "override"
`)

	cfg, err := LoadFromPaths(globalPath, projPath)
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}

	// Override with no project hook = nothing runs
	assertSlice(t, "PostCreate", cfg.Hooks.PostCreate, nil)
}

func TestLoadFromPaths_PartialConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeTOML(t, dir, "partial.toml", `
[hooks]
on_open = "open.sh"
`)

	cfg, err := LoadFromPaths(path, "")
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}

	assertSlice(t, "OnOpen", cfg.Hooks.OnOpen, []string{"open.sh"})
	if cfg.ShowPath() != DefaultShowPath {
		t.Errorf("ShowPath() = %v, want default %v", cfg.ShowPath(), DefaultShowPath)
	}
	if cfg.DefaultPathDir() != DefaultDefPath {
		t.Errorf("DefaultPathDir() = %q, want default %q", cfg.DefaultPathDir(), DefaultDefPath)
	}
}

func TestLoadFromPaths_EmptyTOMLFile(t *testing.T) {
	dir := t.TempDir()
	path := writeTOML(t, dir, "empty.toml", "")

	cfg, err := LoadFromPaths(path, "")
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}
	if cfg.ShellCmd() != DefaultShell {
		t.Errorf("ShellCmd() = %q, want default %q", cfg.ShellCmd(), DefaultShell)
	}
}

func TestMergeHook(t *testing.T) {
	tests := []struct {
		name    string
		global  string
		project string
		mode    HookMode
		want    []string
	}{
		{"chain both", "g", "p", HookModeChain, []string{"g", "p"}},
		{"chain global only", "g", "", HookModeChain, []string{"g"}},
		{"chain project only", "", "p", HookModeChain, []string{"p"}},
		{"chain neither", "", "", HookModeChain, nil},
		{"override with project", "g", "p", HookModeOverride, []string{"p"}},
		{"override no project", "g", "", HookModeOverride, nil},
		{"disable", "g", "p", HookModeDisable, nil},
		{"disable no hooks", "", "", HookModeDisable, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeHook(tt.global, tt.project, tt.mode)
			assertSlice(t, tt.name, got, tt.want)
		})
	}
}

func TestParseHookMode(t *testing.T) {
	tests := []struct {
		input string
		want  HookMode
	}{
		{"chain", HookModeChain},
		{"override", HookModeOverride},
		{"disable", HookModeDisable},
		{"", HookModeChain},
		{"bogus", HookModeChain},
	}

	for _, tt := range tests {
		got := parseHookMode(tt.input)
		if got != tt.want {
			t.Errorf("parseHookMode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func assertSlice(t *testing.T, name string, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}
