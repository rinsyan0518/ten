package pathresolve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Env carries every environment-derived directory the rest of the
// pipeline needs. It is resolved once at the composition root (cmd/ten,
// via FromOS) and passed down by value: no package below cmd/ten reads
// the process environment itself, so a stray HOME/XDG_* leak can only
// enter through one place.
type Env struct {
	Home          string
	XDGConfigHome string
	XDGStateHome  string
}

// FromOS resolves Env from the process environment for the given home:
// $XDG_CONFIG_HOME falling back to home/.config, $XDG_STATE_HOME falling
// back to home/.local/state. It is the only function in this package —
// and the only place below cmd/ten — that reads environment variables.
func FromOS(home string) Env {
	env := Env{
		Home:          home,
		XDGConfigHome: filepath.Join(home, ".config"),
		XDGStateHome:  filepath.Join(home, ".local", "state"),
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		env.XDGConfigHome = v
	}
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		env.XDGStateHome = v
	}
	return env
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

// EqualPaths reports whether two path spellings name the same path after
// lexical cleaning (trailing slashes, ".." and "." components, doubled
// separators). It is the comparison every symlink-identity check goes
// through, so a link written with a different but equivalent spelling —
// by hand, or by an earlier ten — is still recognized as ten's own
// instead of being backed up and replaced (apply) or refused (destroy).
// Purely lexical: it does not resolve symlinks in either argument.
func EqualPaths(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
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
