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
	PreApply  string
	Links     []LinkResult
	Templates []TemplateResult
	PostApply string
}

func (o ToolOutcome) empty() bool {
	return o.PreApply == "" && o.PostApply == "" && len(o.Links) == 0 && len(o.Templates) == 0
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
// running pre_apply/links/templates/post_apply for each. It always
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
			fmt.Fprintf(p.Out, "warning: skipping prune of %s: %s\n", target, result.SkipReason)
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
	for _, name := range order {
		tool := p.Merged.Tools[name]
		outcome := ToolOutcome{Tool: name, PreApply: tool.PreApply}
		fail := func(err error) (Result, state.State, error) {
			if !outcome.empty() {
				outcomes = append(outcomes, outcome)
			}
			return Result{Outcomes: outcomes, Prunes: prunes}, newState, fmt.Errorf("apply: tool %s: %w", name, err)
		}

		if err := p.Executor.RunHook(tool.PreApply, p.Out, p.DryRun); err != nil {
			return fail(err)
		}

		for _, d := range byTool[name] {
			switch d.Kind {
			case "symlink":
				result, err := p.Executor.Link(d.Target, d.Source, backupDir, p.DryRun)
				if err != nil {
					return fail(err)
				}
				if !p.DryRun {
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
				alreadyManaged := p.Current.ManagedResources[d.Target].Type == "template"
				result, err := p.Executor.RenderTemplate(d.Target, d.Source, p.Merged.Vars, backupDir, alreadyManaged, p.DryRun)
				if err != nil {
					return fail(err)
				}
				if !p.DryRun {
					backupPath := result.BackupPath
					if backupPath == "" {
						backupPath = p.Current.ManagedResources[d.Target].BackupPath
					}
					newState.ManagedResources[d.Target] = state.Resource{Tool: name, Type: "template", Source: d.Source, BackupPath: backupPath}
				}
				outcome.Templates = append(outcome.Templates, result)
			}
		}

		if err := p.Executor.RunHook(tool.PostApply, p.Out, p.DryRun); err != nil {
			return fail(err)
		}
		outcome.PostApply = tool.PostApply
		if !outcome.empty() {
			outcomes = append(outcomes, outcome)
		}
	}

	return Result{Outcomes: outcomes, Prunes: prunes}, newState, nil
}
