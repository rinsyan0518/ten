package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/pathresolve"
	"github.com/rinsyan0518/ten/internal/plan"
	"github.com/rinsyan0518/ten/internal/render"
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
	env := pathresolve.FromOS(home)

	current, statePath, err := loadBootstrap(env)
	if err != nil {
		return fmt.Errorf("apply: %w", err)
	}

	ten, err := render.NewSystemInfo(home, current.Profile, current.DotfilesRoot)
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

	pl, err := plan.Build(plan.BuildParams{
		Merged:    merged,
		Current:   current,
		Env:       env,
		Inspector: apply.NewOSInspector(),
	})
	if err != nil {
		return fmt.Errorf("apply: %w", err)
	}

	// --dry-run stops at the plan: nothing below this line runs, so a
	// dry-run structurally cannot touch the filesystem or the state file.
	if dryRun {
		_, _ = fmt.Fprint(cmd.OutOrStdout(), formatPlan(pl))
		return nil
	}

	result, newState, runErr := apply.Execute(apply.ExecParams{
		Plan:      pl,
		Current:   current,
		BackupDir: filepath.Join(home, ".ten_backup"),
		Vars:      merged.Vars,
		Ten:       ten,
		HookDir:   merged.DotfilesRoot,
		Out:       cmd.OutOrStdout(),
		Executor:  apply.NewOSExecutor(),
	})
	_, _ = fmt.Fprint(cmd.OutOrStdout(), formatApplyPlan(result))

	// apply.Execute builds newState from scratch (ManagedResources only);
	// it doesn't know about the bootstrap fields, so carry them over
	// explicitly or a saved ten.state.json would lose dotfiles_root/
	// profile after every apply, forcing a re-run of `ten init`.
	newState.DotfilesRoot = current.DotfilesRoot
	newState.Profile = current.Profile
	// LastApplied records the last apply that fully succeeded; a failed
	// run keeps the previous timestamp so the field stays useful for
	// diagnosing "when did this machine last converge".
	if runErr == nil {
		newState.LastApplied = time.Now()
	} else {
		newState.LastApplied = current.LastApplied
	}
	if saveErr := state.Save(statePath, newState); saveErr != nil && runErr == nil {
		return fmt.Errorf("apply: save state: %w", saveErr)
	}
	return runErr
}
