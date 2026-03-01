package config

import (
	"os"
	"path/filepath"
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

	// Hooks
	if cfg.Hooks.PreCreate != "echo creating" {
		t.Errorf("PreCreate = %q, want %q", cfg.Hooks.PreCreate, "echo creating")
	}
	if cfg.Hooks.PostCreate != "echo created" {
		t.Errorf("PostCreate = %q, want %q", cfg.Hooks.PostCreate, "echo created")
	}
	if cfg.Hooks.OnOpen != "./scripts/open.sh" {
		t.Errorf("OnOpen = %q, want %q", cfg.Hooks.OnOpen, "./scripts/open.sh")
	}
	// Unset hooks should be empty
	if cfg.Hooks.PreDelete != "" {
		t.Errorf("PreDelete = %q, want empty", cfg.Hooks.PreDelete)
	}

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
	// Both files missing — should return empty config with defaults, no error.
	cfg, err := LoadFromPaths("/nonexistent/global.toml", "/nonexistent/project.toml")
	if err != nil {
		t.Fatalf("expected no error for missing files, got: %v", err)
	}
	if cfg.ShellCmd() != DefaultShell {
		t.Errorf("ShellCmd() = %q, want default %q", cfg.ShellCmd(), DefaultShell)
	}
}

func TestLoadFromPaths_EmptyPaths(t *testing.T) {
	// Empty string paths — should skip both, return defaults.
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
	if cfg.Hooks.OnOpen != "global-open" {
		t.Errorf("OnOpen = %q, want %q", cfg.Hooks.OnOpen, "global-open")
	}
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
	if cfg.Hooks.OnOpen != "project-open" {
		t.Errorf("OnOpen = %q, want %q", cfg.Hooks.OnOpen, "project-open")
	}
	if cfg.ShowPath() != false {
		t.Errorf("ShowPath() = %v, want false", cfg.ShowPath())
	}
	// Unset fields should still return defaults
	if cfg.ShellCmd() != DefaultShell {
		t.Errorf("ShellCmd() = %q, want default %q", cfg.ShellCmd(), DefaultShell)
	}
}

func TestLoadFromPaths_MergeBehavior(t *testing.T) {
	dir := t.TempDir()

	globalPath := writeTOML(t, dir, "global.toml", `
[hooks]
pre_create = "global-pre-create"
on_open = "global-open"
on_switch = "global-switch"

[display]
show_path = true
path_style = "relative"

[general]
default_path = "global-wt"
shell = "bash -c"
`)

	projPath := writeTOML(t, dir, "project.toml", `
[hooks]
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

	tests := []struct {
		name string
		got  string
		want string
	}{
		// Project overrides global
		{"OnOpen (project overrides)", cfg.Hooks.OnOpen, "project-open"},
		{"DefaultPathDir (project overrides)", cfg.DefaultPathDir(), "worktrees"},
		// Global value preserved when project doesn't set it
		{"PreCreate (global preserved)", cfg.Hooks.PreCreate, "global-pre-create"},
		{"OnSwitch (global preserved)", cfg.Hooks.OnSwitch, "global-switch"},
		{"ShellCmd (global preserved)", cfg.ShellCmd(), "bash -c"},
		// Global path_style preserved (project didn't set it)
		{"PathStyle (global preserved)", cfg.PathStyle(), "relative"},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}

	// Bool field: project overrides global
	if cfg.ShowPath() != false {
		t.Errorf("ShowPath() = %v, want false (project override)", cfg.ShowPath())
	}
}

func TestMerge_AllHooks(t *testing.T) {
	dst := &Config{
		Hooks: HooksConfig{
			PreCreate:  "dst-pre-create",
			PostCreate: "dst-post-create",
			PreDelete:  "dst-pre-delete",
			PostDelete: "dst-post-delete",
			OnSwitch:   "dst-on-switch",
			OnOpen:     "dst-on-open",
			PrePrune:   "dst-pre-prune",
			PostPrune:  "dst-post-prune",
		},
	}

	src := &Config{
		Hooks: HooksConfig{
			PreCreate: "src-pre-create",
			OnOpen:    "src-on-open",
			// All others empty — should NOT override dst
		},
	}

	merge(dst, src)

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"PreCreate (overridden)", dst.Hooks.PreCreate, "src-pre-create"},
		{"OnOpen (overridden)", dst.Hooks.OnOpen, "src-on-open"},
		{"PostCreate (preserved)", dst.Hooks.PostCreate, "dst-post-create"},
		{"PreDelete (preserved)", dst.Hooks.PreDelete, "dst-pre-delete"},
		{"PostDelete (preserved)", dst.Hooks.PostDelete, "dst-post-delete"},
		{"OnSwitch (preserved)", dst.Hooks.OnSwitch, "dst-on-switch"},
		{"PrePrune (preserved)", dst.Hooks.PrePrune, "dst-pre-prune"},
		{"PostPrune (preserved)", dst.Hooks.PostPrune, "dst-post-prune"},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestMerge_DisplayPointers(t *testing.T) {
	dst := &Config{
		Display: DisplayConfig{
			ShowPath:  boolPtr(true),
			PathStyle: strPtr("relative"),
		},
	}

	// src overrides ShowPath to false, leaves PathStyle nil (should not override)
	src := &Config{
		Display: DisplayConfig{
			ShowPath: boolPtr(false),
		},
	}

	merge(dst, src)

	if *dst.Display.ShowPath != false {
		t.Errorf("ShowPath = %v, want false", *dst.Display.ShowPath)
	}
	if *dst.Display.PathStyle != "relative" {
		t.Errorf("PathStyle = %q, want %q", *dst.Display.PathStyle, "relative")
	}
}

func TestMerge_GeneralPointers(t *testing.T) {
	dst := &Config{
		General: GeneralConfig{
			DefaultPath: strPtr("global-wt"),
			Shell:       strPtr("bash -c"),
		},
	}

	// src overrides Shell, leaves DefaultPath nil
	src := &Config{
		General: GeneralConfig{
			Shell: strPtr("zsh -c"),
		},
	}

	merge(dst, src)

	if *dst.General.DefaultPath != "global-wt" {
		t.Errorf("DefaultPath = %q, want %q", *dst.General.DefaultPath, "global-wt")
	}
	if *dst.General.Shell != "zsh -c" {
		t.Errorf("Shell = %q, want %q", *dst.General.Shell, "zsh -c")
	}
}

func TestLoadFromPaths_PartialConfig(t *testing.T) {
	// Config with only some sections — others should use defaults.
	dir := t.TempDir()
	path := writeTOML(t, dir, "partial.toml", `
[hooks]
on_open = "open.sh"
`)

	cfg, err := LoadFromPaths(path, "")
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}

	if cfg.Hooks.OnOpen != "open.sh" {
		t.Errorf("OnOpen = %q, want %q", cfg.Hooks.OnOpen, "open.sh")
	}
	// Display and General should be nil pointers → defaults
	if cfg.ShowPath() != DefaultShowPath {
		t.Errorf("ShowPath() = %v, want default %v", cfg.ShowPath(), DefaultShowPath)
	}
	if cfg.DefaultPathDir() != DefaultDefPath {
		t.Errorf("DefaultPathDir() = %q, want default %q", cfg.DefaultPathDir(), DefaultDefPath)
	}
}

func TestLoadFromPaths_EmptyTOMLFile(t *testing.T) {
	// Completely empty file — should parse fine, all defaults.
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