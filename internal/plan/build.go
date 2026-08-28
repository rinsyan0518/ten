package plan

import (
	"fmt"
	"sort"

	"github.com/rinsyan0518/ten/internal/config"
	"github.com/rinsyan0518/ten/internal/graph"
	"github.com/rinsyan0518/ten/internal/pathresolve"
	"github.com/rinsyan0518/ten/internal/state"
)

// Action is what one plan step will do to its target when executed.
type Action string

const (
	// ActionCreate creates a resource where nothing exists yet.
	ActionCreate Action = "create"
	// ActionReplace backs up whatever currently occupies the target,
	// then puts ten's resource in its place.
	ActionReplace Action = "replace"
	// ActionUpdate overwrites ten's own previous output in place, with
	// no backup (templates only).
	ActionUpdate Action = "update"
	// ActionNoop leaves an already-correct resource untouched.
	ActionNoop Action = "noop"
	// ActionRemove deletes a resource that left ten's control.
	ActionRemove Action = "remove"
	// ActionRestore puts the recorded backup back over the target.
	ActionRestore Action = "restore"
	// ActionSkip touches nothing because the user replaced ten's
	// resource with something of their own; SkipReason says why.
	ActionSkip Action = "skip"
)

// Entry is what Inspector.Inspect saw at a path, without following
// symlinks.
type Entry struct {
	Exists    bool
	IsSymlink bool
	LinkDest  string // symlink destination; meaningful only when IsSymlink
}

// Inspector is the read-only filesystem probe Build plans against. The
// real implementation lstats the disk; tests substitute a fake. Build
// performs no writes through it — a plan is a prediction, never a side
// effect.
type Inspector interface {
	Inspect(path string) (Entry, error)
}

// LinkStep is one planned symlink.
type LinkStep struct {
	Target string
	Source string
	Action Action // create | replace | noop
}

// TemplateStep is one planned template render. Rendering itself happens
// at execution time, after the owning tool's before hook has run — a
// hook may generate or update the template source, so rendering earlier
// would bake in stale content. The plan therefore never predicts noop
// for a template; execution reports one when the rendered output turns
// out identical to what is on disk.
type TemplateStep struct {
	Target string
	Source string
	Action Action // create | replace | update
}

// PruneStep is one resource leaving ten's control this run.
type PruneStep struct {
	Target     string
	Type       string // "symlink" | "template"
	BackupPath string
	Action     Action // remove | restore | skip
	SkipReason string
}

// ToolPlan groups one tool's planned steps, in execution order:
// Before, Links/Templates, Once (if armed), After.
type ToolPlan struct {
	Tool      string
	Before    string
	Links     []LinkStep
	Templates []TemplateStep
	// Once is non-empty only when the tool defines a once hook and this
	// run newly manages at least one of the tool's targets (i.e. a
	// desired target absent from Current.ManagedResources).
	Once  string
	After string
}

func (t ToolPlan) empty() bool {
	return t.Before == "" && t.Once == "" && t.After == "" && len(t.Links) == 0 && len(t.Templates) == 0
}

// Plan is everything one apply run intends to do, computed up front
// against the current state file and a read-only look at the disk.
// Prunes execute first, then Tools in dependency order.
type Plan struct {
	Prunes []PruneStep
	Tools  []ToolPlan
}

// BuildParams configures one Build call.
type BuildParams struct {
	Merged    config.Merged
	Current   state.State
	Env       pathresolve.Env
	Inspector Inspector
}

// Build computes the full plan for one apply run: dependency order via
// graph.Sort, desired targets via Desired (including duplicate-target
// detection), prune steps for state entries no longer desired, and a
// per-resource action derived from what is actually on disk. Build only
// reads — through p.Inspector — and never mutates filesystem or state.
//
// Existence of link/template sources is deliberately NOT checked here:
// a tool's before hook is allowed to generate its sources, and hooks
// run at execution time. Missing sources stay an execution-time error.
func Build(p BuildParams) (Plan, error) {
	order, err := graph.Sort(p.Merged)
	if err != nil {
		return Plan{}, err
	}

	desired, err := Desired(p.Merged, order, p.Env)
	if err != nil {
		return Plan{}, err
	}
	desiredSet := make(map[string]bool, len(desired))
	for _, d := range desired {
		desiredSet[d.Target] = true
	}

	pruneTargets := Prune(p.Current, desiredSet)
	sort.Strings(pruneTargets)
	prunes := make([]PruneStep, 0, len(pruneTargets))
	for _, target := range pruneTargets {
		step, err := planPrune(target, p.Current.ManagedResources[target], p.Inspector)
		if err != nil {
			return Plan{}, err
		}
		prunes = append(prunes, step)
	}

	byTool := make(map[string][]Target)
	for _, d := range desired {
		byTool[d.Tool] = append(byTool[d.Tool], d)
	}

	var tools []ToolPlan
	for _, name := range order {
		tool := p.Merged.Tools[name]
		tp := ToolPlan{Tool: name, Before: tool.Before, After: tool.After}

		newlyManaged := false
		for _, d := range byTool[name] {
			if _, tracked := p.Current.ManagedResources[d.Target]; !tracked {
				newlyManaged = true
			}
			switch d.Kind {
			case "symlink":
				step, err := planLink(d, p.Inspector)
				if err != nil {
					return Plan{}, fmt.Errorf("tool %s: %w", name, err)
				}
				tp.Links = append(tp.Links, step)
			case "template":
				alreadyManaged := p.Current.ManagedResources[d.Target].Type == "template"
				step, err := planTemplate(d, alreadyManaged, p.Inspector)
				if err != nil {
					return Plan{}, fmt.Errorf("tool %s: %w", name, err)
				}
				tp.Templates = append(tp.Templates, step)
			}
		}

		if newlyManaged && tool.Once != "" {
			tp.Once = tool.Once
		}
		if !tp.empty() {
			tools = append(tools, tp)
		}
	}

	return Plan{Prunes: prunes, Tools: tools}, nil
}

func planLink(d Target, ins Inspector) (LinkStep, error) {
	entry, err := ins.Inspect(d.Target)
	if err != nil {
		return LinkStep{}, fmt.Errorf("inspect %s: %w", d.Target, err)
	}
	step := LinkStep{Target: d.Target, Source: d.Source}
	switch {
	case !entry.Exists:
		step.Action = ActionCreate
	case entry.IsSymlink && entry.LinkDest == d.Source:
		step.Action = ActionNoop
	default:
		step.Action = ActionReplace
	}
	return step, nil
}

func planTemplate(d Target, alreadyManaged bool, ins Inspector) (TemplateStep, error) {
	entry, err := ins.Inspect(d.Target)
	if err != nil {
		return TemplateStep{}, fmt.Errorf("inspect %s: %w", d.Target, err)
	}
	step := TemplateStep{Target: d.Target, Source: d.Source}
	switch {
	case !entry.Exists:
		step.Action = ActionCreate
	case !alreadyManaged:
		// Whatever occupies the target is not ten's template output —
		// back it up before writing, exactly like a link would.
		step.Action = ActionReplace
	default:
		step.Action = ActionUpdate
	}
	return step, nil
}

func planPrune(target string, res state.Resource, ins Inspector) (PruneStep, error) {
	step := PruneStep{Target: target, Type: res.Type, BackupPath: res.BackupPath}

	entry, err := ins.Inspect(target)
	if err != nil {
		return PruneStep{}, fmt.Errorf("inspect %s: %w", target, err)
	}
	// Only a symlink can be verified as still being ten's: it must still
	// point at the recorded source. Template output is a plain file with
	// no such marker. A missing target skips verification — there is
	// nothing left to protect.
	if entry.Exists && res.Type == "symlink" {
		switch {
		case !entry.IsSymlink:
			step.Action = ActionSkip
			step.SkipReason = "no longer a symlink created by ten"
			return step, nil
		case entry.LinkDest != res.Source:
			step.Action = ActionSkip
			step.SkipReason = fmt.Sprintf("symlink now points at %s, not %s", entry.LinkDest, res.Source)
			return step, nil
		}
	}

	if res.BackupPath != "" {
		// Whether the recorded backup still exists is checked at
		// execution time by Unlink, right before the target is removed
		// (fail-fast with partial progress) — not here, so a single
		// missing backup can't block the whole run from starting.
		step.Action = ActionRestore
		return step, nil
	}
	step.Action = ActionRemove
	return step, nil
}
