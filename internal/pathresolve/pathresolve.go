package pathresolve

import (
	"fmt"
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
