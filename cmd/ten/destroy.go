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
			result, err := apply.Unlink(apply.UnlinkRequest{
				Target:     target,
				Type:       res.Type,
				Source:     res.Source,
				BackupPath: res.BackupPath,
			}, dryRun)
			if err != nil {
				if !dryRun {
					if saveErr := state.Save(statePath, remaining); saveErr != nil {
						return fmt.Errorf("destroy: tool %s: %w (also failed to save partial state: %v)", tool, err, saveErr)
					}
				}
				return fmt.Errorf("destroy: tool %s: %w", tool, err)
			}
			// A skipped resource was left untouched on disk, so it stays
			// tracked for a human to resolve rather than being quietly
			// dropped from state.
			if !dryRun && !result.Skipped {
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

// unlinkLine renders one resource leaving ten's control, in the format
// shared by destroy's plan and apply's Prune section (§5): a restore names
// the backup it comes from, a removal names the resource type.
func unlinkLine(r apply.UnlinkResult, resType, backupPath, suffix string) string {
	if r.Restored {
		return fmt.Sprintf("    + restore backup%s    %s <- %s\n", suffix, r.Target, backupPath)
	}
	kind := resType
	if kind == "" {
		kind = "resource"
	}
	return fmt.Sprintf("    - remove %s%s    %s\n", kind, suffix, r.Target)
}

func printUnlinkResult(cmd *cobra.Command, tool string, r apply.UnlinkResult, dryRun bool) {
	if r.Skipped {
		fmt.Fprintf(cmd.OutOrStdout(), "warning: skipping %s: %s\n", r.Target, r.SkipReason)
		return
	}
	verb := "- remove"
	if r.Restored {
		verb = "+ restore backup"
	}
	if dryRun {
		verb += " (dry-run)"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  %s\n    %s   %s\n", tool, verb, r.Target)
}
