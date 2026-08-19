package apply

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"github.com/rinsyan0518/ten/internal/config"
	"github.com/rinsyan0518/ten/internal/graph"
	"github.com/rinsyan0518/ten/internal/plan"
	"github.com/rinsyan0518/ten/internal/state"
)

// RunParams configures a single ten apply run.
type RunParams struct {
	Merged   config.Merged
	Current  state.State
	Home     string
	DryRun   bool
	Out      io.Writer
	Executor Executor
}

// ToolOutcome is what one tool did during an apply run: which hooks ran
// and which resources were created or rendered.
type ToolOutcome struct {
	Tool      string
	Before    string
	Links     []LinkResult
	Templates []TemplateResult
	// Once is non-empty only when the tool's once hook actually fired (or,
	// under --dry-run, was eligible to fire) this run.
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

// Result is everything that happened during an Apply run, in display
// order, regardless of whether the run succeeded or stopped early.
type Result struct {
	Outcomes []ToolOutcome
	Prunes   []PruneOutcome
}

// Apply resolves p.Merged's desired resources against p.Current, prunes
// whatever is no longer desired, then walks tools in dependency order
// running before/links/templates/after for each. It always
// returns the Result and state.State reflecting everything it actually
// did, even when it returns a non-nil error: the first failing step
// stops the run (fail-fast), but everything before it is kept in both
// the returned Result and state.State. Apply performs no state I/O; the
// caller is responsible for loading Current and saving the returned
// state.State.
func Apply(p RunParams) (Result, state.State, error) {
	order, err := graph.Sort(p.Merged)
	if err != nil {
		return Result{}, p.Current, fmt.Errorf("apply: %w", err)
	}

	out := p.Out
	if out == nil {
		out = io.Discard
	}

	desired, err := plan.Desired(p.Merged, order, p.Home)
	if err != nil {
		return Result{}, p.Current, fmt.Errorf("apply: %w", err)
	}

	backupDir := filepath.Join(p.Home, ".ten_backup")

	desiredSet := make(map[string]bool, len(desired))
	for _, d := range desired {
		desiredSet[d.Target] = true
	}

	pruneTargets := plan.Prune(p.Current, desiredSet)
	sort.Strings(pruneTargets)

	// Seed the new state from every currently tracked resource, so anything
	// this run never reaches — because an earlier tool or prune failed
	// first — keeps its prior record instead of being silently dropped from
	// tracking. Entries are removed only once their prune has actually
	// succeeded, and desired entries are rewritten as they are applied.
	newState := state.State{ManagedResources: make(map[string]state.Resource, len(p.Current.ManagedResources))}
	for target, res := range p.Current.ManagedResources {
		newState.ManagedResources[target] = res
	}

	var prunes []PruneOutcome
	for _, target := range pruneTargets {
		res := p.Current.ManagedResources[target]
		result, err := p.Executor.Unlink(UnlinkRequest{
			Target:     target,
			Type:       res.Type,
			Source:     res.Source,
			BackupPath: res.BackupPath,
		}, p.DryRun)
		if err != nil {
			return Result{Prunes: prunes}, newState, fmt.Errorf("apply: prune %s: %w", target, err)
		}
		if result.Skipped {
			_, _ = fmt.Fprintf(out, "warning: skipping prune of %s: %s\n", target, result.SkipReason)
			continue
		}
		if !p.DryRun {
			delete(newState.ManagedResources, target)
		}
		prunes = append(prunes, PruneOutcome{Result: result, Type: res.Type, BackupPath: res.BackupPath})
	}

	byTool := make(map[string][]plan.Target)
	for _, d := range desired {
		byTool[d.Tool] = append(byTool[d.Tool], d)
	}

	var outcomes []ToolOutcome
	// Iterate the full DAG order rather than only the tools that own
	// resources, so a tool defining just hooks still runs them in
	// dependency order.
	for _, name := range order {
		tool := p.Merged.Tools[name]
		outcome := ToolOutcome{Tool: name, Before: tool.Before}
		fail := func(err error) (Result, state.State, error) {
			if !outcome.empty() {
				outcomes = append(outcomes, outcome)
			}
			return Result{Outcomes: outcomes, Prunes: prunes}, newState, fmt.Errorf("apply: tool %s: %w", name, err)
		}

		if err := p.Executor.RunHook(tool.Before, out, p.DryRun); err != nil {
			return fail(err)
		}

		// newlyManaged tracks whether this run causes the tool to manage a
		// target it didn't already own, per p.Current (the state as it was
		// when this run started) — independent of whether the filesystem
		// operation itself was a no-op (LinkResult.Skipped). This is the
		// trigger for the once hook below.
		newlyManaged := false
		for _, d := range byTool[name] {
			if _, tracked := p.Current.ManagedResources[d.Target]; !tracked {
				newlyManaged = true
			}
			switch d.Kind {
			case "symlink":
				result, err := p.Executor.Link(d.Target, d.Source, backupDir, p.DryRun)
				if err != nil {
					return fail(err)
				}
				if !p.DryRun {
					// A backup is only produced the run that first takes it;
					// a later idempotent apply returns no BackupPath even
					// though the earlier backup (and the state entry
					// pointing at it) is still what destroy must restore.
					// Fall back to whatever was already recorded for this
					// target so a routine re-apply doesn't clobber it.
					backupPath := result.BackupPath
					if backupPath == "" {
						backupPath = p.Current.ManagedResources[d.Target].BackupPath
					}
					newState.ManagedResources[d.Target] = state.Resource{Tool: name, Type: "symlink", Source: d.Source, BackupPath: backupPath}
				}
				if !result.Skipped {
					outcome.Links = append(outcome.Links, result)
				}
			case "template":
				// Only a resource previously managed as a template may be
				// overwritten without a backup. Mere key presence is not
				// enough: a target converted from links to templates is a
				// symlink into the dotfiles repo, and treating it as ten's
				// own output would render straight through it.
				alreadyManaged := p.Current.ManagedResources[d.Target].Type == "template"
				result, err := p.Executor.RenderTemplate(d.Target, d.Source, p.Merged.Vars, backupDir, alreadyManaged, p.DryRun)
				if err != nil {
					return fail(err)
				}
				if !p.DryRun {
					// Same fallback as the symlink branch above: a
					// re-render of an already-managed template takes no
					// fresh backup, so preserve whatever backup_path was
					// already on record for this target.
					backupPath := result.BackupPath
					if backupPath == "" {
						backupPath = p.Current.ManagedResources[d.Target].BackupPath
					}
					newState.ManagedResources[d.Target] = state.Resource{Tool: name, Type: "template", Source: d.Source, BackupPath: backupPath}
				}
				outcome.Templates = append(outcome.Templates, result)
			}
		}

		if newlyManaged && tool.Once != "" {
			if err := p.Executor.RunHook(tool.Once, out, p.DryRun); err != nil {
				return fail(err)
			}
			outcome.Once = tool.Once
		}

		if err := p.Executor.RunHook(tool.After, out, p.DryRun); err != nil {
			return fail(err)
		}
		outcome.After = tool.After
		if !outcome.empty() {
			outcomes = append(outcomes, outcome)
		}
	}

	return Result{Outcomes: outcomes, Prunes: prunes}, newState, nil
}
