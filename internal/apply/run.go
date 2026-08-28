package apply

import (
	"fmt"
	"io"
	"maps"

	"github.com/rinsyan0518/ten/internal/plan"
	"github.com/rinsyan0518/ten/internal/state"
)

// ExecParams configures a single execution of an apply plan.
type ExecParams struct {
	Plan      plan.Plan
	Current   state.State
	BackupDir string
	// Vars and Ten feed template rendering, which happens here rather
	// than at plan time: a tool's before hook may generate or update the
	// template source, so rendering must wait until after it has run.
	Vars     map[string]string
	Ten      SystemInfo // Tool is zero-valued here; Execute fills it per tool
	Out      io.Writer
	Executor Executor
}

// ToolOutcome is what one tool did during an apply run: which hooks ran
// and which resources were created, rendered, or found up to date.
type ToolOutcome struct {
	Tool      string
	Before    string
	Links     []LinkResult
	Templates []TemplateResult
	// Once is non-empty only when the tool's once hook actually fired
	// this run.
	Once  string
	After string
}

func (o ToolOutcome) empty() bool {
	return o.Before == "" && o.Once == "" && o.After == "" && len(o.Links) == 0 && len(o.Templates) == 0
}

// PruneOutcome is one resource that left ten's control this run, plus the
// state record needed to describe what happened to it.
type PruneOutcome struct {
	Result     UnlinkResult
	Type       string
	BackupPath string
}

// Result is everything that happened during an Execute run, in display
// order, regardless of whether the run succeeded or stopped early.
type Result struct {
	Outcomes []ToolOutcome
	Prunes   []PruneOutcome
}

// Execute performs the side effects a plan.Build plan describes: prunes
// first, then each tool's before/links/templates/once/after in plan
// order. Noop and skip steps never reach the Executor. It always returns
// the Result and state.State reflecting everything it actually did, even
// when it returns a non-nil error: the first failing step stops the run
// (fail-fast), but everything before it is kept in both. Execute
// performs no state I/O; the caller loads Current and saves the returned
// state.State.
func Execute(p ExecParams) (Result, state.State, error) {
	out := p.Out
	if out == nil {
		out = io.Discard
	}

	// Seed the new state from every currently tracked resource, so anything
	// this run never reaches — because an earlier tool or prune failed
	// first — keeps its prior record instead of being silently dropped from
	// tracking. Entries are removed only once their prune has actually
	// succeeded, and desired entries are rewritten as they are applied.
	newState := state.State{ManagedResources: make(map[string]state.Resource, len(p.Current.ManagedResources))}
	maps.Copy(newState.ManagedResources, p.Current.ManagedResources)

	var prunes []PruneOutcome
	for _, step := range p.Plan.Prunes {
		if step.Action == plan.ActionSkip {
			_, _ = fmt.Fprintf(out, "warning: skipping prune of %s: %s\n", step.Target, step.SkipReason)
			continue
		}
		res := p.Current.ManagedResources[step.Target]
		result, err := p.Executor.Unlink(UnlinkRequest{
			Target:     step.Target,
			Type:       step.Type,
			Source:     res.Source,
			BackupPath: step.BackupPath,
		})
		if err != nil {
			return Result{Prunes: prunes}, newState, fmt.Errorf("apply: prune %s: %w", step.Target, err)
		}
		// Unlink re-verifies ownership at execution time; a skip here means
		// the disk changed between plan and execution.
		if result.Skipped {
			_, _ = fmt.Fprintf(out, "warning: skipping prune of %s: %s\n", step.Target, result.SkipReason)
			continue
		}
		delete(newState.ManagedResources, step.Target)
		prunes = append(prunes, PruneOutcome{Result: result, Type: step.Type, BackupPath: step.BackupPath})
	}

	var outcomes []ToolOutcome
	for _, tp := range p.Plan.Tools {
		outcome := ToolOutcome{Tool: tp.Tool, Before: tp.Before}
		fail := func(err error) (Result, state.State, error) {
			if !outcome.empty() {
				outcomes = append(outcomes, outcome)
			}
			return Result{Outcomes: outcomes, Prunes: prunes}, newState, fmt.Errorf("apply: tool %s: %w", tp.Tool, err)
		}

		if err := p.Executor.RunHook(tp.Before, out); err != nil {
			return fail(err)
		}

		for _, step := range tp.Links {
			if step.Action == plan.ActionNoop {
				outcome.Links = append(outcome.Links, LinkResult{Target: step.Target, Source: step.Source, Skipped: true})
				continue
			}
			result, err := p.Executor.Link(step.Target, step.Source, p.BackupDir)
			if err != nil {
				return fail(err)
			}
			// A backup is only produced the run that first takes it; a later
			// idempotent apply returns no BackupPath even though the earlier
			// backup (and the state entry pointing at it) is still what
			// destroy must restore. Fall back to whatever was already
			// recorded for this target.
			backupPath := result.BackupPath
			if backupPath == "" {
				backupPath = p.Current.ManagedResources[step.Target].BackupPath
			}
			newState.ManagedResources[step.Target] = state.Resource{Tool: tp.Tool, Type: "symlink", Source: step.Source, BackupPath: backupPath}
			outcome.Links = append(outcome.Links, result)
		}

		for _, step := range tp.Templates {
			ten := p.Ten
			ten.Tool = tp.Tool
			result, err := p.Executor.RenderTemplate(step.Target, step.Source, p.Vars, ten, p.BackupDir, step.Action == plan.ActionReplace)
			if err != nil {
				return fail(err)
			}
			// Same fallback as the symlink branch above: a re-render of an
			// already-managed template takes no fresh backup, so preserve
			// whatever backup_path was already on record for this target.
			backupPath := result.BackupPath
			if backupPath == "" {
				backupPath = p.Current.ManagedResources[step.Target].BackupPath
			}
			newState.ManagedResources[step.Target] = state.Resource{Tool: tp.Tool, Type: "template", Source: step.Source, BackupPath: backupPath}
			outcome.Templates = append(outcome.Templates, result)
		}

		if err := p.Executor.RunHook(tp.Once, out); err != nil {
			// Roll back tracking of targets this run newly managed for
			// this tool (the resources themselves stay on disk). Recording
			// them would make the next run consider them already tracked
			// and never re-fire once — the failed setup command would be
			// silently lost forever. Untracked, the next apply re-links
			// them idempotently and arms once again.
			for _, step := range tp.Links {
				if _, tracked := p.Current.ManagedResources[step.Target]; !tracked {
					delete(newState.ManagedResources, step.Target)
				}
			}
			for _, step := range tp.Templates {
				if _, tracked := p.Current.ManagedResources[step.Target]; !tracked {
					delete(newState.ManagedResources, step.Target)
				}
			}
			return fail(err)
		}
		outcome.Once = tp.Once

		if err := p.Executor.RunHook(tp.After, out); err != nil {
			return fail(err)
		}
		outcome.After = tp.After
		if !outcome.empty() {
			outcomes = append(outcomes, outcome)
		}
	}

	return Result{Outcomes: outcomes, Prunes: prunes}, newState, nil
}
