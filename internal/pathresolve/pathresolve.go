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

// Resolve converts a prefixed key like "home:.gitconfig", "xdg:nvim", or
// "custom:/etc/foo" into an absolute path.
func Resolve(env Env, key string) (string, error) {
	switch {
	case strings.HasPrefix(key, "home:"):
		return filepath.Join(env.Home, strings.TrimPrefix(key, "home:")), nil
	case strings.HasPrefix(key, "xdg:"):
		return filepath.Join(env.XDGConfigHome, strings.TrimPrefix(key, "xdg:")), nil
	case strings.HasPrefix(key, "custom:"):
		p := strings.TrimPrefix(key, "custom:")
		if strings.HasPrefix(p, "~/") {
			return filepath.Join(env.Home, strings.TrimPrefix(p, "~/")), nil
		}
		if !filepath.IsAbs(p) {
			return "", fmt.Errorf("pathresolve: custom path must be absolute: %q", p)
		}
		return filepath.Clean(p), nil
	default:
		return "", fmt.Errorf("pathresolve: unknown prefix in key %q", key)
	}
}

// ResolveKey resolves a prefixed key against home, reading
// $XDG_CONFIG_HOME (falling back to home/.config) for the xdg: prefix.
func ResolveKey(home, key string) (string, error) {
	return Resolve(Env{Home: home, XDGConfigHome: XDGConfigHome(home)}, key)
}

// XDGConfigHome returns $XDG_CONFIG_HOME, falling back to home/.config.
func XDGConfigHome(home string) string {
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
