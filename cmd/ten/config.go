package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/rinsyan0518/ten/internal/config"
	"github.com/rinsyan0518/ten/internal/pathresolve"
)

// loadMerged loads ten.local.toml and ten.toml under home and returns the
// merged configuration. It is the single source of truth for config
// loading shared by the apply and destroy commands.
func loadMerged(home string) (config.Merged, error) {
	localPath := filepath.Join(home, ".config", "ten", "ten.local.toml")
	local, err := config.LoadLocal(localPath)
	if err != nil {
		return config.Merged{}, err
	}

	dotfilesRoot := expandHome(local.Core.DotfilesRoot, home)
	base, _, err := config.LoadRepo(filepath.Join(dotfilesRoot, "ten.toml"))
	if err != nil {
		return config.Merged{}, err
	}

	var profilePtr *config.Repo
	if local.Core.Profile != "" {
		profileRepo, ok, err := config.LoadRepo(filepath.Join(dotfilesRoot, "ten."+local.Core.Profile+".toml"))
		if err != nil {
			return config.Merged{}, err
		}
		if ok {
			profilePtr = &profileRepo
		}
	}

	merged, err := config.Merge(base, profilePtr, local)
	if err != nil {
		return config.Merged{}, err
	}
	merged.DotfilesRoot = dotfilesRoot
	return merged, nil
}

func expandHome(p, home string) string {
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, strings.TrimPrefix(p, "~/"))
	}
	return p
}

func resolveKey(key, home string) (string, error) {
	env := pathresolve.Env{Home: home, XDGConfigHome: xdgConfigHome(home)}
	return pathresolve.Resolve(env, key)
}

func xdgConfigHome(home string) string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	return filepath.Join(home, ".config")
}
