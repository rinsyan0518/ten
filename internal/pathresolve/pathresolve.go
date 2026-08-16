package pathresolve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Env carries the directories needed to resolve prefixed path keys.
type Env struct {
	Home          string
	XDGConfigHome string
}

// Resolve converts a prefixed key like "home:.gitconfig" or "xdg:nvim" into
// an absolute path.
func Resolve(env Env, key string) (string, error) {
	switch {
	case strings.HasPrefix(key, "home:"):
		return filepath.Join(env.Home, strings.TrimPrefix(key, "home:")), nil
	case strings.HasPrefix(key, "xdg:"):
		return filepath.Join(env.XDGConfigHome, strings.TrimPrefix(key, "xdg:")), nil
	default:
		return "", fmt.Errorf("pathresolve: unknown prefix in key %q", key)
	}
}

// ExpandHome expands a leading "~" or "~/" in path to home. A path that
// doesn't start with either is returned unchanged.
func ExpandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return path
}

// ResolveKey resolves a prefixed key against home, reading
// $XDG_CONFIG_HOME (falling back to home/.config) for the xdg: prefix.
func ResolveKey(home, key string) (string, error) {
	return Resolve(Env{Home: home, XDGConfigHome: xdgConfigHome(home)}, key)
}

// xdgConfigHome returns $XDG_CONFIG_HOME, falling back to home/.config.
func xdgConfigHome(home string) string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	return filepath.Join(home, ".config")
}

// XDGStateHome returns $XDG_STATE_HOME, falling back to home/.local/state.
func XDGStateHome(home string) string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v
	}
	return filepath.Join(home, ".local", "state")
}
