package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// HooksConfig maps hook event names to shell commands.
type HooksConfig struct {
	PreCreate  string `toml:"pre_create"`
	PostCreate string `toml:"post_create"`
	PreDelete  string `toml:"pre_delete"`
	PostDelete string `toml:"post_delete"`
	OnSwitch   string `toml:"on_switch"`
	OnOpen     string `toml:"on_open"`
	PrePrune   string `toml:"pre_prune"`
	PostPrune  string `toml:"post_prune"`
}

// DisplayConfig controls how worktrees are rendered in the TUI.
type DisplayConfig struct {
	ShowPath  *bool   `toml:"show_path"`  // nil = not set (defaults to true)
	PathStyle *string `toml:"path_style"` // nil = not set (defaults to "relative")
}

// GeneralConfig holds general runtime settings.
type GeneralConfig struct {
	DefaultPath *string `toml:"default_path"` // nil = not set (defaults to "worktrees")
	Shell       *string `toml:"shell"`        // nil = not set (defaults to "sh -c")
}

// Config is the top-level configuration, composed of all sections.
type Config struct {
	Hooks   HooksConfig   `toml:"hooks"`
	Display DisplayConfig `toml:"display"`
	General GeneralConfig `toml:"general"`
}

// Default values
const (
	DefaultShowPath  = true
	DefaultPathStyle = "relative"
	DefaultDefPath   = "worktrees"
	DefaultShell     = "sh -c"
)

// GlobalConfigDir returns the path to the global config directory.
// Uses $XDG_CONFIG_HOME if set, otherwise falls back to ~/.config.
func GlobalConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "lazywt"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "lazywt"), nil
}

// GlobalConfigPath returns the path to the global config file.
func GlobalConfigPath() (string, error) {
	dir, err := GlobalConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// ShowPath returns the effective show_path value, applying defaults.
func (c *Config) ShowPath() bool {
	if c.Display.ShowPath != nil {
		return *c.Display.ShowPath
	}
	return DefaultShowPath
}

// PathStyle returns the effective path_style value, applying defaults.
func (c *Config) PathStyle() string {
	if c.Display.PathStyle != nil {
		return *c.Display.PathStyle
	}
	return DefaultPathStyle
}

// DefaultPathDir returns the effective default_path value, applying defaults.
func (c *Config) DefaultPathDir() string {
	if c.General.DefaultPath != nil {
		return *c.General.DefaultPath
	}
	return DefaultDefPath
}

// ShellCmd returns the effective shell value, applying defaults.
func (c *Config) ShellCmd() string {
	if c.General.Shell != nil {
		return *c.General.Shell
	}
	return DefaultShell
}

// Load reads configuration from the global config file and the project
// config file, merging them with project values taking precedence.
// Both files are optional — missing files are silently ignored.
// projectPath should be the path to the project lazywt.toml file.
func Load(projectPath string) (*Config, error) {
	cfg := &Config{}

	// Load global config first
	globalPath, err := GlobalConfigPath()
	if err != nil {
		// Can't determine config dir — skip global, not fatal
		globalPath = ""
	}

	if globalPath != "" {
		global, err := loadFile(globalPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if global != nil {
			cfg = global
		}
	}

	// Load project config and merge (project overrides global)
	if projectPath != "" {
		project, err := loadFile(projectPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if project != nil {
			merge(cfg, project)
		}
	}

	return cfg, nil
}

// LoadFromPaths reads config from explicit global and project paths.
// This is the testable core of Load — both paths are fully controlled by the caller.
// Pass empty string for either path to skip it.
func LoadFromPaths(globalPath, projectPath string) (*Config, error) {
	cfg := &Config{}

	if globalPath != "" {
		global, err := loadFile(globalPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if global != nil {
			cfg = global
		}
	}

	if projectPath != "" {
		project, err := loadFile(projectPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if project != nil {
			merge(cfg, project)
		}
	}

	return cfg, nil
}

// loadFile reads and parses a single TOML config file.
// Returns nil, os.ErrNotExist if the file doesn't exist.
func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// merge overlays src onto dst. Non-nil pointer fields in src override dst.
// String hook fields override if non-empty.
func merge(dst, src *Config) {
	// Hooks: non-empty strings in src override dst
	if src.Hooks.PreCreate != "" {
		dst.Hooks.PreCreate = src.Hooks.PreCreate
	}
	if src.Hooks.PostCreate != "" {
		dst.Hooks.PostCreate = src.Hooks.PostCreate
	}
	if src.Hooks.PreDelete != "" {
		dst.Hooks.PreDelete = src.Hooks.PreDelete
	}
	if src.Hooks.PostDelete != "" {
		dst.Hooks.PostDelete = src.Hooks.PostDelete
	}
	if src.Hooks.OnSwitch != "" {
		dst.Hooks.OnSwitch = src.Hooks.OnSwitch
	}
	if src.Hooks.OnOpen != "" {
		dst.Hooks.OnOpen = src.Hooks.OnOpen
	}
	if src.Hooks.PrePrune != "" {
		dst.Hooks.PrePrune = src.Hooks.PrePrune
	}
	if src.Hooks.PostPrune != "" {
		dst.Hooks.PostPrune = src.Hooks.PostPrune
	}

	// Display: non-nil pointers in src override dst
	if src.Display.ShowPath != nil {
		dst.Display.ShowPath = src.Display.ShowPath
	}
	if src.Display.PathStyle != nil {
		dst.Display.PathStyle = src.Display.PathStyle
	}

	// General: non-nil pointers in src override dst
	if src.General.DefaultPath != nil {
		dst.General.DefaultPath = src.General.DefaultPath
	}
	if src.General.Shell != nil {
		dst.General.Shell = src.General.Shell
	}
}