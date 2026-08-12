package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/plan"
	"github.com/rinsyan0518/ten/internal/state"
	"github.com/spf13/cobra"
)

func newDestroyCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "destroy",
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
	merged, err := loadMerged(home)
	if err != nil {
		return fmt.Errorf("destroy: load config: %w", err)
	}

	managed := make(map[string]bool, len(st.ManagedResources))
	byTool := make(map[string][]string)
	for target, res := range st.ManagedResources {
		managed[res.Tool] = true
		byTool[res.Tool] = append(byTool[res.Tool], target)
	}
	// st.ManagedResources is a map, so byTool's slices are built in a
	// nondeterministic order; sort them for a reproducible destroy order
	// per tool (same convention as runApply's link/template key sorting).
	for tool := range byTool {
		sort.Strings(byTool[tool])
	}

	order, err := plan.DestroyOrder(merged.Tools, managed)
	if err != nil {
		return fmt.Errorf("destroy: %w", err)
	}

	// Seed remaining from the full current state, so any resource this run
	// never reaches (a tool not yet processed, or a target whose removal
	// hasn't been attempted yet) keeps its prior record instead of being
	// silently dropped from tracking. Entries are removed only once their
	// removal/restore has actually succeeded.
	remaining := state.State{LastApplied: st.LastApplied, ManagedResources: make(map[string]state.Resource, len(st.ManagedResources))}
	for target, res := range st.ManagedResources {
		remaining.ManagedResources[target] = res
	}

	for _, tool := range order {
		for _, target := range byTool[tool] {
			res := st.ManagedResources[target]
			result, err := apply.Unlink(target, res.BackupPath, dryRun)
			if err != nil {
				if !dryRun {
					if saveErr := state.Save(statePath, remaining); saveErr != nil {
						return fmt.Errorf("destroy: tool %s: %w (also failed to save partial state: %v)", tool, err, saveErr)
					}
				}
				return fmt.Errorf("destroy: tool %s: %w", tool, err)
			}
			if !dryRun {
				delete(remaining.ManagedResources, target)
			}
			printUnlinkResult(cmd, tool, result, dryRun)
		}
	}

	if !dryRun {
		if err := state.Save(statePath, remaining); err != nil {
			return fmt.Errorf("destroy: save state: %w", err)
		}
	}
	return nil
}

func printUnlinkResult(cmd *cobra.Command, tool string, r apply.UnlinkResult, dryRun bool) {
	verb := "- remove"
	if r.Restored {
		verb = "+ restore backup"
	}
	if dryRun {
		verb += " (dry-run)"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  %s\n    %s   %s\n", tool, verb, r.Target)
}
