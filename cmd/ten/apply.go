package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/pathresolve"
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

	merged, repoFound, err := loadMerged(home)
	if err != nil {
		return fmt.Errorf("apply: load config: %w", err)
	}
	if err := checkDesiredState(merged, repoFound); err != nil {
		return fmt.Errorf("apply: %w", err)
	}

	statePath := filepath.Join(pathresolve.XDGStateHome(home), "ten", "ten.state.json")
	current, err := state.Load(statePath)
	if err != nil {
		return fmt.Errorf("apply: load state: %w", err)
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
		newState.LastApplied = time.Now()
		if saveErr := state.Save(statePath, newState); saveErr != nil && runErr == nil {
			return fmt.Errorf("apply: save state: %w", saveErr)
		}
	}
	return runErr
}
