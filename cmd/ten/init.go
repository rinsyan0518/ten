package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rinsyan0518/ten/internal/pathresolve"
	"github.com/rinsyan0518/ten/internal/state"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var path, profile string
	cmd := &cobra.Command{
		Use:   "init",
		Args:  cobra.NoArgs,
		Short: "Point ten at a dotfiles repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, path, profile)
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "path to the dotfiles repository (defaults to the current directory)")
	cmd.Flags().StringVar(&profile, "profile", "", "profile to activate; leaves the existing profile unchanged if omitted, clears it if passed as an empty string")
	return cmd
}

func runInit(cmd *cobra.Command, path, profile string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("init: resolve home dir: %w", err)
	}

	if path == "" {
		path, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("init: resolve current directory: %w", err)
		}
	} else {
		path = pathresolve.ExpandHome(path, home)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("init: resolve path %s: %w", path, err)
	}
	if info, statErr := os.Stat(absPath); statErr != nil {
		return fmt.Errorf("init: %s is not an existing directory: %w", absPath, statErr)
	} else if !info.IsDir() {
		return fmt.Errorf("init: %s is not an existing directory", absPath)
	}
	// Store the symlink-resolved form so different spellings of the same
	// directory (a symlinked path, /tmp vs /private/tmp on macOS, …)
	// converge on one canonical dotfiles_root. Symlink identity checks
	// compare against this root; re-initializing through another spelling
	// would otherwise make every recorded symlink look foreign and get
	// replaced on the next apply.
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		absPath = resolved
	}

	statePath := statePathFor(pathresolve.FromOS(home))
	st, err := state.Load(statePath)
	if err != nil {
		return fmt.Errorf("init: load state: %w", err)
	}

	st.DotfilesRoot = absPath
	if cmd.Flags().Changed("profile") {
		st.Profile = profile
	}

	if err := state.Save(statePath, st); err != nil {
		return fmt.Errorf("init: save state: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Initialized ten for %s", absPath)
	if st.Profile != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), " (profile: %s)", st.Profile)
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout())
	return nil
}
