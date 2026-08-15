package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/state"
	"github.com/spf13/cobra"
)

func newDestroyCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "destroy",
		Args:  cobra.NoArgs,
		Short: "Remove all resources ten currently manages",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDestroy(cmd, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan without changing anything")
	return cmd
}

func runDestroy(cmd *cobra.Command, dryRun bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("destroy: resolve home dir: %w", err)
	}
	statePath := filepath.Join(home, ".config", "ten", "ten.state.json")

	st, err := state.Load(statePath)
	if err != nil {
		return fmt.Errorf("destroy: load state: %w", err)
	}
	merged, _, err := loadMerged(home)
	if err != nil {
		return fmt.Errorf("destroy: load config: %w", err)
	}

	result, remaining, runErr := apply.Destroy(apply.DestroyParams{
		Merged:   merged,
		Current:  st,
		Home:     home,
		DryRun:   dryRun,
		Out:      cmd.OutOrStdout(),
		Executor: apply.NewOSExecutor(),
	})
	_, _ = fmt.Fprint(cmd.OutOrStdout(), formatDestroyPlan(result, dryRun))

	if !dryRun {
		if saveErr := state.Save(statePath, remaining); saveErr != nil {
			if runErr != nil {
				return fmt.Errorf("%w (also failed to save partial state: %v)", runErr, saveErr)
			}
			return fmt.Errorf("destroy: save state: %w", saveErr)
		}
	}
	return runErr
}
