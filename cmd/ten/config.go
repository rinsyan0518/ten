package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rinsyan0518/ten/internal/config"
)

// loadMerged loads ten.local.toml and ten.toml under home and returns the
// merged configuration. It is the single source of truth for config
// loading shared by the apply and destroy commands. repoFound reports
// whether any repository config file (ten.toml or ten.<profile>.toml) was
// actually present; apply uses it as a safety check (see checkDesiredState).
func loadMerged(home string) (merged config.Merged, repoFound bool, err error) {
	localPath := filepath.Join(home, ".config", "ten", "ten.local.toml")
	local, err := config.LoadLocal(localPath)
	if err != nil {
		return config.Merged{}, false, err
	}

	dotfilesRoot := expandHome(local.Core.DotfilesRoot, home)
	base, baseFound, err := config.LoadRepo(filepath.Join(dotfilesRoot, "ten.toml"))
	if err != nil {
		return config.Merged{}, false, err
	}
	repoFound = baseFound

	var profilePtr *config.Repo
	if local.Core.Profile != "" {
		profileRepo, ok, err := config.LoadRepo(filepath.Join(dotfilesRoot, "ten."+local.Core.Profile+".toml"))
		if err != nil {
			return config.Merged{}, false, err
		}
		if ok {
			profilePtr = &profileRepo
			repoFound = true
		}
	}

	merged, err = config.Merge(base, profilePtr, local)
	if err != nil {
		return config.Merged{}, false, err
	}
	merged.DotfilesRoot = dotfilesRoot
	return merged, repoFound, nil
}

// checkDesiredState guards against a desired state that is empty by
// accident rather than by intent (§4-④): an unset or missing
// dotfiles_root, or a dotfiles root with no repo config in it at all.
// Without this, `ten apply` on a machine where the dotfiles repo isn't
// cloned yet would prune every managed resource.
func checkDesiredState(merged config.Merged, repoFound bool) error {
	if merged.DotfilesRoot == "" {
		return fmt.Errorf("core.dotfiles_root is not set in ten.local.toml")
	}
	info, err := os.Stat(merged.DotfilesRoot)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("core.dotfiles_root %s is not an existing directory", merged.DotfilesRoot)
	}
	if !repoFound {
		return fmt.Errorf("no ten.toml or ten.<profile>.toml found under %s", merged.DotfilesRoot)
	}
	return nil
}

func expandHome(p, home string) string {
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, strings.TrimPrefix(p, "~/"))
	}
	return p
}
