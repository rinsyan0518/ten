package main

import (
	"fmt"
	"os"
	"time"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/state"
	"github.com/spf13/cobra"
)

func newApplyCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "apply",
		Args:  cobra.NoArgs,
		Short: "Apply the desired dotfiles state",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan without changing anything")
	return cmd
}

func runApply(cmd *cobra.Command, dryRun bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("apply: resolve home dir: %w", err)
	}

	current, statePath, err := loadBootstrap(home)
	if err != nil {
		return fmt.Errorf("apply: %w", err)
	}

	merged, repoFound, err := loadMerged(current.DotfilesRoot, current.Profile)
	if err != nil {
		return fmt.Errorf("apply: load config: %w", err)
	}
	if err := checkDesiredState(merged, repoFound); err != nil {
		return fmt.Errorf("apply: %w", err)
	}

	result, newState, runErr := apply.Apply(apply.RunParams{
		Merged:   merged,
		Current:  current,
		Home:     home,
		DryRun:   dryRun,
		Out:      cmd.OutOrStdout(),
		Executor: apply.NewOSExecutor(),
	})
	_, _ = fmt.Fprint(cmd.OutOrStdout(), formatApplyPlan(result, dryRun))

	if !dryRun {
		// apply.Apply builds newState from scratch (ManagedResources only);
		// it doesn't know about the bootstrap fields, so carry them over
		// explicitly or a saved ten.state.json would lose dotfiles_root/
		// profile after every apply, forcing a re-run of `ten init`.
		newState.DotfilesRoot = current.DotfilesRoot
		newState.Profile = current.Profile
		newState.LastApplied = time.Now()
		if saveErr := state.Save(statePath, newState); saveErr != nil && runErr == nil {
			return fmt.Errorf("apply: save state: %w", saveErr)
		}
	}
	return runErr
}
