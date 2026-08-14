package apply

import (
	"fmt"
	"io"
	"sort"

	"github.com/rinsyan0518/ten/internal/config"
	"github.com/rinsyan0518/ten/internal/plan"
	"github.com/rinsyan0518/ten/internal/state"
)

// DestroyParams configures a single ten destroy run.
type DestroyParams struct {
	Merged   config.Merged
	Current  state.State
	Home     string // unused by Destroy itself; kept for symmetry with RunParams
	DryRun   bool
	Out      io.Writer
	Executor Executor
}

// DestroyEntry is one resource destroy took back out of ten's control,
// plus the state record needed to describe what happened to it.
type DestroyEntry struct {
	Result     UnlinkResult
	Type       string
	BackupPath string
}

// DestroyOutcome groups one tool's destroyed resources, mirroring
// Apply's per-tool grouping.
type DestroyOutcome struct {
	Tool    string
	Entries []DestroyEntry
}

// DestroyResult is everything that happened during a Destroy run, in
// display order, regardless of whether the run succeeded or stopped
// early.
type DestroyResult struct {
	Outcomes []DestroyOutcome
}

// Destroy removes every resource recorded in p.Current, in reverse
// dependency order (the mirror of Apply's order). It always returns the
// DestroyResult and state.State reflecting everything actually removed
// or restored so far, even when it returns a non-nil error (fail-fast).
// Destroy performs no state I/O; the caller is responsible for loading
// Current and saving the returned state.State. Destroy does not update
// state.State.LastApplied.
func Destroy(p DestroyParams) (DestroyResult, state.State, error) {
	managed := make(map[string]bool, len(p.Current.ManagedResources))
	byTool := make(map[string][]string)
	for target, res := range p.Current.ManagedResources {
		managed[res.Tool] = true
		byTool[res.Tool] = append(byTool[res.Tool], target)
	}
	// p.Current.ManagedResources is a map, so byTool's slices are built in
	// a nondeterministic order; sort them for a reproducible destroy order
	// per tool (same convention as plan.Desired's link/template key
	// sorting).
	for tool := range byTool {
		sort.Strings(byTool[tool])
	}

	order, err := plan.DestroyOrder(p.Merged.Tools, managed)
	if err != nil {
		return DestroyResult{}, p.Current, fmt.Errorf("destroy: %w", err)
	}

	out := p.Out
	if out == nil {
		out = io.Discard
	}

	remaining := state.State{LastApplied: p.Current.LastApplied, ManagedResources: make(map[string]state.Resource, len(p.Current.ManagedResources))}
	for target, res := range p.Current.ManagedResources {
		remaining.ManagedResources[target] = res
	}

	var outcomes []DestroyOutcome
	for _, tool := range order {
		outcome := DestroyOutcome{Tool: tool}
		for _, target := range byTool[tool] {
			res := p.Current.ManagedResources[target]
			result, err := p.Executor.Unlink(UnlinkRequest{
				Target:     target,
				Type:       res.Type,
				Source:     res.Source,
				BackupPath: res.BackupPath,
			}, p.DryRun)
			if err != nil {
				if len(outcome.Entries) > 0 {
					outcomes = append(outcomes, outcome)
				}
				return DestroyResult{Outcomes: outcomes}, remaining, fmt.Errorf("destroy: tool %s: %w", tool, err)
			}
			if result.Skipped {
				fmt.Fprintf(out, "warning: skipping %s: %s\n", result.Target, result.SkipReason)
				continue
			}
			if !p.DryRun {
				delete(remaining.ManagedResources, target)
			}
			outcome.Entries = append(outcome.Entries, DestroyEntry{Result: result, Type: res.Type, BackupPath: res.BackupPath})
		}
		if len(outcome.Entries) > 0 {
			outcomes = append(outcomes, outcome)
		}
	}

	return DestroyResult{Outcomes: outcomes}, remaining, nil
}
