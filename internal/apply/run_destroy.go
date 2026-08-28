package apply

import (
	"fmt"
	"io"
	"maps"

	"github.com/rinsyan0518/ten/internal/plan"
	"github.com/rinsyan0518/ten/internal/state"
)

// DestroyExecParams configures a single execution of a destroy plan.
type DestroyExecParams struct {
	Plan    plan.DestroyPlan
	Current state.State
	// BackupDir bounds the empty-directory cleanup after each restored
	// backup (the backup dir under the XDG state dir); empty disables the cleanup.
	BackupDir string
	Out       io.Writer
	Executor  Executor
}

// DestroyEntry is one resource destroy took back out of ten's control,
// plus the state record needed to describe what happened to it.
type DestroyEntry struct {
	Result     UnlinkResult
	Type       string
	BackupPath string
}

// DestroyOutcome groups one tool's destroyed resources, mirroring
// Execute's per-tool grouping.
type DestroyOutcome struct {
	Tool    string
	Entries []DestroyEntry
}

// DestroyResult is everything that happened during an ExecuteDestroy
// run, in display order, regardless of whether the run succeeded or
// stopped early.
type DestroyResult struct {
	Outcomes []DestroyOutcome
}

// ExecuteDestroy performs the removals a plan.BuildDestroy plan
// describes, in plan order. Skip steps never reach the Executor. It
// always returns the DestroyResult and state.State reflecting everything
// actually removed or restored so far, even when it returns a non-nil
// error (fail-fast). ExecuteDestroy performs no state I/O; the caller
// loads Current and saves the returned state.State. It does not update
// state.State.LastApplied.
func ExecuteDestroy(p DestroyExecParams) (DestroyResult, state.State, error) {
	out := p.Out
	if out == nil {
		out = io.Discard
	}

	remaining := state.State{LastApplied: p.Current.LastApplied, ManagedResources: make(map[string]state.Resource, len(p.Current.ManagedResources))}
	maps.Copy(remaining.ManagedResources, p.Current.ManagedResources)

	var outcomes []DestroyOutcome
	for _, tp := range p.Plan.Tools {
		outcome := DestroyOutcome{Tool: tp.Tool}
		for _, step := range tp.Steps {
			if step.Action == plan.ActionSkip {
				_, _ = fmt.Fprintf(out, "warning: skipping %s: %s\n", step.Target, step.SkipReason)
				continue
			}
			res := p.Current.ManagedResources[step.Target]
			result, err := p.Executor.Unlink(UnlinkRequest{
				Target:      step.Target,
				Type:        step.Type,
				Source:      res.Source,
				BackupPath:  step.BackupPath,
				ContentHash: res.ContentHash,
				BackupRoot:  p.BackupDir,
			})
			if err != nil {
				if len(outcome.Entries) > 0 {
					outcomes = append(outcomes, outcome)
				}
				return DestroyResult{Outcomes: outcomes}, remaining, fmt.Errorf("destroy: tool %s: %w", tp.Tool, err)
			}
			// Unlink re-verifies ownership at execution time; a skip here
			// means the disk changed between plan and execution.
			if result.Skipped {
				_, _ = fmt.Fprintf(out, "warning: skipping %s: %s\n", result.Target, result.SkipReason)
				continue
			}
			delete(remaining.ManagedResources, step.Target)
			outcome.Entries = append(outcome.Entries, DestroyEntry{Result: result, Type: step.Type, BackupPath: step.BackupPath})
		}
		if len(outcome.Entries) > 0 {
			outcomes = append(outcomes, outcome)
		}
	}

	return DestroyResult{Outcomes: outcomes}, remaining, nil
}
