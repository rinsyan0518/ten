package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/plan"
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

	out := cmd.OutOrStdout()
	var outcomes []destroyOutcome
	for _, tool := range order {
		outcome := destroyOutcome{Tool: tool}
		for _, target := range byTool[tool] {
			res := st.ManagedResources[target]
			result, err := apply.Unlink(apply.UnlinkRequest{
				Target:     target,
				Type:       res.Type,
				Source:     res.Source,
				BackupPath: res.BackupPath,
			}, dryRun)
			if err != nil {
				// Report what was already destroyed before stopping, the
				// same way a failed apply does.
				if len(outcome.Entries) > 0 {
					outcomes = append(outcomes, outcome)
				}
				fmt.Fprint(out, formatDestroyPlan(outcomes, dryRun))
				if !dryRun {
					if saveErr := state.Save(statePath, remaining); saveErr != nil {
						return fmt.Errorf("destroy: tool %s: %w (also failed to save partial state: %v)", tool, err, saveErr)
					}
				}
				return fmt.Errorf("destroy: tool %s: %w", tool, err)
			}
			if result.Skipped {
				// Left untouched on disk, so it stays tracked for a human
				// to resolve rather than being quietly dropped from state.
				fmt.Fprintf(out, "warning: skipping %s: %s\n", result.Target, result.SkipReason)
				continue
			}
			if !dryRun {
				delete(remaining.ManagedResources, target)
			}
			outcome.Entries = append(outcome.Entries, destroyEntry{Result: result, Type: res.Type, BackupPath: res.BackupPath})
		}
		if len(outcome.Entries) > 0 {
			outcomes = append(outcomes, outcome)
		}
	}

	fmt.Fprint(out, formatDestroyPlan(outcomes, dryRun))

	if !dryRun {
		if err := state.Save(statePath, remaining); err != nil {
			return fmt.Errorf("destroy: save state: %w", err)
		}
	}
	return nil
}

// destroyEntry is one resource destroy took back out of ten's control,
// plus the state record needed to describe what happened to it.
type destroyEntry struct {
	Result     apply.UnlinkResult
	Type       string
	BackupPath string
}

// destroyOutcome groups one tool's destroyed resources, mirroring apply's
// per-tool grouping.
type destroyOutcome struct {
	Tool    string
	Entries []destroyEntry
}

// formatDestroyPlan renders the destroy plan in the §5 format: a summary
// line, then each tool's resources in destroy order.
func formatDestroyPlan(outcomes []destroyOutcome, dryRun bool) string {
	total := 0
	for _, o := range outcomes {
		total += len(o.Entries)
	}
	if total == 0 {
		return "Plan: 0 to destroy\n"
	}

	suffix := ""
	if dryRun {
		suffix = " (dry-run)"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Plan: %d to destroy\n\n", total)
	for _, o := range outcomes {
		fmt.Fprintf(&b, "  %s\n", o.Tool)
		for _, e := range o.Entries {
			b.WriteString(unlinkLine(e.Result, e.Type, e.BackupPath, suffix))
		}
		b.WriteString("\n")
	}
	return b.String()
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
