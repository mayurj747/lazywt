package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// HookMode controls how global and project hooks are combined.
type HookMode string

const (
	HookModeChain    HookMode = "chain"    // run global then local (default)
	HookModeOverride HookMode = "override" // local replaces global
	HookModeDisable  HookMode = "disable"  // no hook runs
)

// rawHooksModeConfig holds per-hook mode overrides from TOML.
type rawHooksModeConfig struct {
	PreCreate  string `toml:"pre_create"`
	PostCreate string `toml:"post_create"`
	PreDelete  string `toml:"pre_delete"`
	PostDelete string `toml:"post_delete"`
	OnSwitch   string `toml:"on_switch"`
	OnOpen     string `toml:"on_open"`
	PrePrune   string `toml:"pre_prune"`
	PostPrune  string `toml:"post_prune"`
}

// rawHooksConfig is the TOML-parsed hooks section.
type rawHooksConfig struct {
	PreCreate  string             `toml:"pre_create"`
	PostCreate string             `toml:"post_create"`
	PreDelete  string             `toml:"pre_delete"`
	PostDelete string             `toml:"post_delete"`
	OnSwitch   string             `toml:"on_switch"`
	OnOpen     string             `toml:"on_open"`
	PrePrune   string             `toml:"pre_prune"`
	PostPrune  string             `toml:"post_prune"`
	Mode       rawHooksModeConfig `toml:"mode"`
}

// rawConfig is the TOML-parsed configuration before merging.
type rawConfig struct {
	Hooks   rawHooksConfig `toml:"hooks"`
	Display DisplayConfig  `toml:"display"`
	General GeneralConfig  `toml:"general"`
}

// HooksConfig holds resolved hook command chains.
type HooksConfig struct {
	PreCreate  []string
	PostCreate []string
	PreDelete  []string
	PostDelete []string
	OnSwitch   []string
	OnOpen     []string
	PrePrune   []string
	PostPrune  []string
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
	Hooks   HooksConfig
	Display DisplayConfig
	General GeneralConfig
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
// config file, merging them with hook chaining by default.
// Both files are optional — missing files are silently ignored.
// projectPath should be the path to the project lazywt.toml file.
func Load(projectPath string) (*Config, error) {
	globalPath, err := GlobalConfigPath()
	if err != nil {
		globalPath = ""
	}
	return LoadFromPaths(globalPath, projectPath)
}

// LoadFromPaths reads config from explicit global and project paths.
// This is the testable core of Load — both paths are fully controlled by the caller.
// Pass empty string for either path to skip it.
func LoadFromPaths(globalPath, projectPath string) (*Config, error) {
	var global, project *rawConfig

	if globalPath != "" {
		raw, err := loadRawFile(globalPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		global = raw
	}

	if projectPath != "" {
		raw, err := loadRawFile(projectPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		project = raw
	}

	return mergeAndResolve(global, project), nil
}

// loadRawFile reads and parses a single TOML config file into a rawConfig.
func loadRawFile(path string) (*rawConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw rawConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return &raw, nil
}

// mergeAndResolve combines global and project rawConfigs into a resolved Config.
func mergeAndResolve(global, project *rawConfig) *Config {
	cfg := &Config{}

	var g, p rawHooksConfig
	if global != nil {
		g = global.Hooks
		cfg.Display = global.Display
		cfg.General = global.General
	}
	if project != nil {
		p = project.Hooks
		mergeDisplay(&cfg.Display, &project.Display)
		mergeGeneral(&cfg.General, &project.General)
	}

	cfg.Hooks = HooksConfig{
		PreCreate:  mergeHook(g.PreCreate, p.PreCreate, parseHookMode(p.Mode.PreCreate)),
		PostCreate: mergeHook(g.PostCreate, p.PostCreate, parseHookMode(p.Mode.PostCreate)),
		PreDelete:  mergeHook(g.PreDelete, p.PreDelete, parseHookMode(p.Mode.PreDelete)),
		PostDelete: mergeHook(g.PostDelete, p.PostDelete, parseHookMode(p.Mode.PostDelete)),
		OnSwitch:   mergeHook(g.OnSwitch, p.OnSwitch, parseHookMode(p.Mode.OnSwitch)),
		OnOpen:     mergeHook(g.OnOpen, p.OnOpen, parseHookMode(p.Mode.OnOpen)),
		PrePrune:   mergeHook(g.PrePrune, p.PrePrune, parseHookMode(p.Mode.PrePrune)),
		PostPrune:  mergeHook(g.PostPrune, p.PostPrune, parseHookMode(p.Mode.PostPrune)),
	}

	return cfg
}

// mergeHook combines a global and project hook command according to the mode.
func mergeHook(globalCmd, projectCmd string, mode HookMode) []string {
	switch mode {
	case HookModeDisable:
		return nil
	case HookModeOverride:
		return singleHook(projectCmd)
	default: // chain
		var result []string
		if globalCmd != "" {
			result = append(result, globalCmd)
		}
		if projectCmd != "" {
			result = append(result, projectCmd)
		}
		return result
	}
}

func singleHook(cmd string) []string {
	if cmd == "" {
		return nil
	}
	return []string{cmd}
}

func parseHookMode(s string) HookMode {
	switch HookMode(s) {
	case HookModeOverride:
		return HookModeOverride
	case HookModeDisable:
		return HookModeDisable
	default:
		return HookModeChain
	}
}

func mergeDisplay(dst, src *DisplayConfig) {
	if src.ShowPath != nil {
		dst.ShowPath = src.ShowPath
	}
	if src.PathStyle != nil {
		dst.PathStyle = src.PathStyle
	}
}

func mergeGeneral(dst, src *GeneralConfig) {
	if src.DefaultPath != nil {
		dst.DefaultPath = src.DefaultPath
	}
	if src.Shell != nil {
		dst.Shell = src.Shell
	}
}
