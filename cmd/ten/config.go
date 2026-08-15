package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rinsyan0518/ten/internal/config"
	"github.com/rinsyan0518/ten/internal/pathresolve"
	"github.com/rinsyan0518/ten/internal/state"
)

// statePathFor returns the fixed $XDG_STATE_HOME location of
// ten.state.json for the given home directory.
func statePathFor(home string) string {
	return filepath.Join(pathresolve.XDGStateHome(home), "ten", "ten.state.json")
}

// loadBootstrap reads ten.state.json from its fixed $XDG_STATE_HOME
// location and returns it along with the path it was read from (so
// callers can save back to the same place). It errors if dotfiles_root
// has never been set, i.e. `ten init` has not been run.
func loadBootstrap(home string) (st state.State, statePath string, err error) {
	statePath = statePathFor(home)
	st, err = state.Load(statePath)
	if err != nil {
		return state.State{}, statePath, fmt.Errorf("load state %s: %w", statePath, err)
	}
	if st.DotfilesRoot == "" {
		return state.State{}, statePath, fmt.Errorf("dotfiles_root is not set; run `ten init` inside your dotfiles repository")
	}
	return st, statePath, nil
}

// loadMerged loads ten.local.toml, ten.toml, and ten.<profile>.toml from
// dotfilesRoot and returns the merged configuration. It is the single
// source of truth for config loading shared by the apply and destroy
// commands. repoFound reports whether any repository config file
// (ten.toml or ten.<profile>.toml) was actually present; apply uses it as
// a safety check (see checkDesiredState).
func loadMerged(dotfilesRoot, profile string) (merged config.Merged, repoFound bool, err error) {
	base, baseFound, err := config.LoadFile(filepath.Join(dotfilesRoot, "ten.toml"))
	if err != nil {
		return config.Merged{}, false, err
	}
	repoFound = baseFound

	var profilePtr *config.File
	if profile != "" {
		profileFile, ok, err := config.LoadFile(filepath.Join(dotfilesRoot, "ten."+profile+".toml"))
		if err != nil {
			return config.Merged{}, false, err
		}
		if ok {
			profilePtr = &profileFile
			repoFound = true
		}
	}

	localFile, localFound, err := config.LoadFile(filepath.Join(dotfilesRoot, "ten.local.toml"))
	if err != nil {
		return config.Merged{}, false, err
	}
	var localPtr *config.File
	if localFound {
		localPtr = &localFile
	}

	merged, err = config.Merge(base, profilePtr, localPtr)
	if err != nil {
		return config.Merged{}, false, err
	}
	merged.DotfilesRoot = dotfilesRoot
	return merged, repoFound, nil
}

// checkDesiredState guards against a desired state that is empty by
// accident rather than by intent (§4-④ of the original design doc): a
// dotfiles root that no longer exists, or one with no repo config in it
// at all. Without this, `ten apply` on a machine where the dotfiles repo
// has been moved or deleted since `ten init` would prune every managed
// resource. This is apply-only — destroy deliberately doesn't call it,
// since destroy should still be able to no-op cleanly (or clean up via
// ten.state.json) even when dotfiles_root is gone.
func checkDesiredState(merged config.Merged, repoFound bool) error {
	info, err := os.Stat(merged.DotfilesRoot)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("dotfiles_root %s is not an existing directory", merged.DotfilesRoot)
	}
	if !repoFound {
		return fmt.Errorf("no ten.toml or ten.<profile>.toml found under %s", merged.DotfilesRoot)
	}
	return nil
}
