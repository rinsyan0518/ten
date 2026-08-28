package apply

import (
	"fmt"
	"io"
	"sort"

	"github.com/rinsyan0518/ten/internal/state"
)

// DestroyParams configures a single ten destroy run.
type DestroyParams struct {
	Current  state.State
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

// Destroy removes every resource recorded in p.Current, in a
// deterministic order (tool name, then target name within a tool) derived
// entirely from p.Current — it reads no config. That's safe because
// Destroy never runs hooks (the only thing depends_on orders) and every
// Unlink call is independent of every other tool's state. It always
// returns the DestroyResult and state.State reflecting everything
// actually removed or restored so far, even when it returns a non-nil
// error (fail-fast). Destroy performs no state I/O; the caller is
// responsible for loading Current and saving the returned state.State.
// Destroy does not update state.State.LastApplied.
func Destroy(p DestroyParams) (DestroyResult, state.State, error) {
	byTool := make(map[string][]string)
	for target, res := range p.Current.ManagedResources {
		byTool[res.Tool] = append(byTool[res.Tool], target)
	}
	tools := make([]string, 0, len(byTool))
	for tool := range byTool {
		sort.Strings(byTool[tool])
		tools = append(tools, tool)
	}
	sort.Strings(tools)

	out := p.Out
	if out == nil {
		out = io.Discard
	}

	remaining := state.State{LastApplied: p.Current.LastApplied, ManagedResources: make(map[string]state.Resource, len(p.Current.ManagedResources))}
	for target, res := range p.Current.ManagedResources {
		remaining.ManagedResources[target] = res
	}

	var outcomes []DestroyOutcome
	for _, tool := range tools {
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
				_, _ = fmt.Fprintf(out, "warning: skipping %s: %s\n", result.Target, result.SkipReason)
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
