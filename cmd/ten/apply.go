package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/graph"
	"github.com/rinsyan0518/ten/internal/plan"
	"github.com/rinsyan0518/ten/internal/state"
	"github.com/spf13/cobra"
)

func newApplyCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "apply",
		Args:  cobra.NoArgs,
		Short: "Apply the desired dotfiles state",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan without changing anything")
	return cmd
}

type toolOutcome struct {
	Tool      string
	PreApply  string
	Links     []apply.LinkResult
	Templates []apply.TemplateResult
	PostApply string
}

// pruneOutcome is one resource that left ten's control this run, plus the
// state record needed to describe what happened to it.
type pruneOutcome struct {
	Result     apply.UnlinkResult
	Type       string
	BackupPath string
}

func (o toolOutcome) empty() bool {
	return o.PreApply == "" && o.PostApply == "" && len(o.Links) == 0 && len(o.Templates) == 0
}

func runApply(cmd *cobra.Command, dryRun bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("apply: resolve home dir: %w", err)
	}
	statePath := filepath.Join(home, ".config", "ten", "ten.state.json")

	merged, repoFound, err := loadMerged(home)
	if err != nil {
		return fmt.Errorf("apply: load config: %w", err)
	}
	if err := checkDesiredState(merged, repoFound); err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	order, err := graph.Sort(merged)
	if err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	current, err := state.Load(statePath)
	if err != nil {
		return fmt.Errorf("apply: load state: %w", err)
	}
	backupDir := filepath.Join(home, ".ten_backup")

	// Resolve every desired target up front so prune can run before apply.
	// Link/template keys are sorted per tool for deterministic ordering,
	// matching the convention established in Tasks 5/10/12.
	type desiredTarget struct {
		tool   string
		kind   string // "symlink" | "template"
		target string
		source string
	}
	var desired []desiredTarget
	for _, name := range order {
		tool := merged.Tools[name]

		linkKeys := make([]string, 0, len(tool.Links))
		for k := range tool.Links {
			linkKeys = append(linkKeys, k)
		}
		sort.Strings(linkKeys)
		for _, key := range linkKeys {
			target, err := resolveKey(key, home)
			if err != nil {
				return fmt.Errorf("apply: tool %s: %w", name, err)
			}
			desired = append(desired, desiredTarget{tool: name, kind: "symlink", target: target, source: filepath.Join(merged.DotfilesRoot, tool.Links[key])})
		}

		templateKeys := make([]string, 0, len(tool.Templates))
		for k := range tool.Templates {
			templateKeys = append(templateKeys, k)
		}
		sort.Strings(templateKeys)
		for _, key := range templateKeys {
			target, err := resolveKey(key, home)
			if err != nil {
				return fmt.Errorf("apply: tool %s: %w", name, err)
			}
			desired = append(desired, desiredTarget{tool: name, kind: "template", target: target, source: filepath.Join(merged.DotfilesRoot, tool.Templates[key])})
		}
	}

	desiredSet := make(map[string]bool, len(desired))
	for _, d := range desired {
		desiredSet[d.target] = true
	}

	pruneTargets := plan.Prune(current, desiredSet)
	sort.Strings(pruneTargets) // deterministic prune order/output

	// Seed the new state from every currently tracked resource, so anything
	// this run never reaches — because an earlier tool or prune failed
	// first — keeps its prior record instead of being silently dropped from
	// tracking. Entries are removed only once their prune has actually
	// succeeded, and desired entries are rewritten as they are applied.
	newState := state.State{ManagedResources: make(map[string]state.Resource, len(current.ManagedResources))}
	for target, res := range current.ManagedResources {
		newState.ManagedResources[target] = res
	}

	out := cmd.OutOrStdout()

	var prunes []pruneOutcome
	for _, target := range pruneTargets {
		res := current.ManagedResources[target]
		// Leaving ten's control means the same thing for prune as for
		// destroy (§4-④): restore the backup if there is one, otherwise
		// remove what ten created.
		result, err := apply.Unlink(apply.UnlinkRequest{
			Target:     target,
			Type:       res.Type,
			Source:     res.Source,
			BackupPath: res.BackupPath,
		}, dryRun)
		if err != nil {
			fmt.Fprint(out, formatApplyPlan(nil, prunes, dryRun))
			saveState(statePath, newState, dryRun)
			return fmt.Errorf("apply: prune %s: %w", target, err)
		}
		if result.Skipped {
			// Left untouched on disk, so it stays tracked for a human to
			// resolve rather than being quietly forgotten.
			fmt.Fprintf(out, "warning: skipping prune of %s: %s\n", target, result.SkipReason)
			continue
		}
		if !dryRun {
			delete(newState.ManagedResources, target)
		}
		prunes = append(prunes, pruneOutcome{Result: result, Type: res.Type, BackupPath: res.BackupPath})
	}

	byTool := make(map[string][]desiredTarget)
	for _, d := range desired {
		byTool[d.tool] = append(byTool[d.tool], d)
	}

	var outcomes []toolOutcome
	// Iterate the full DAG order rather than only the tools that own
	// resources, so a tool defining just hooks still runs them in
	// dependency order.
	for _, name := range order {
		tool := merged.Tools[name]
		// Record the outcome as it is built up so a fail-fast exit can
		// still report what this tool got through before it stopped.
		outcome := toolOutcome{Tool: name, PreApply: tool.PreApply}
		fail := func(err error) error {
			if !outcome.empty() {
				outcomes = append(outcomes, outcome)
			}
			// A failed run has already changed the filesystem, so report
			// what got done before stopping rather than leaving the user
			// with only an error message.
			fmt.Fprint(out, formatApplyPlan(outcomes, prunes, dryRun))
			saveState(statePath, newState, dryRun)
			return fmt.Errorf("apply: tool %s: %w", name, err)
		}

		// Hook output streams straight to the user rather than being
		// swallowed; the plan itself is printed after the whole run.
		if err := apply.RunHook(tool.PreApply, out, dryRun); err != nil {
			return fail(err)
		}

		for _, d := range byTool[name] {
			switch d.kind {
			case "symlink":
				result, err := apply.Link(d.target, d.source, backupDir, dryRun)
				if err != nil {
					return fail(err)
				}
				if !dryRun {
					// A backup is only produced the run that first takes it;
					// a later idempotent apply returns no BackupPath even
					// though the earlier backup (and the state entry
					// pointing at it) is still what destroy must restore.
					// Fall back to whatever was already recorded for this
					// target so a routine re-apply doesn't clobber it.
					backupPath := result.BackupPath
					if backupPath == "" {
						backupPath = current.ManagedResources[d.target].BackupPath
					}
					newState.ManagedResources[d.target] = state.Resource{Tool: name, Type: "symlink", Source: d.source, BackupPath: backupPath}
				}
				if !result.Skipped {
					outcome.Links = append(outcome.Links, result)
				}
			case "template":
				// Only a resource previously managed as a template may be
				// overwritten without a backup. Mere key presence is not
				// enough: a target converted from links to templates is a
				// symlink into the dotfiles repo, and treating it as ten's
				// own output would render straight through it (§4-③).
				alreadyManaged := current.ManagedResources[d.target].Type == "template"
				result, err := apply.RenderTemplate(d.target, d.source, merged.Vars, backupDir, alreadyManaged, dryRun)
				if err != nil {
					return fail(err)
				}
				if !dryRun {
					// Same fallback as the symlink branch above: a
					// re-render of an already-managed template takes no
					// fresh backup, so preserve whatever backup_path was
					// already on record for this target.
					backupPath := result.BackupPath
					if backupPath == "" {
						backupPath = current.ManagedResources[d.target].BackupPath
					}
					newState.ManagedResources[d.target] = state.Resource{Tool: name, Type: "template", Source: d.source, BackupPath: backupPath}
				}
				outcome.Templates = append(outcome.Templates, result)
			}
		}

		if err := apply.RunHook(tool.PostApply, out, dryRun); err != nil {
			// PostApply is only claimed in the plan once it has run, so a
			// failing hook is reported by the error, not as a done step.
			return fail(err)
		}
		outcome.PostApply = tool.PostApply
		if !outcome.empty() {
			outcomes = append(outcomes, outcome)
		}
	}

	fmt.Fprint(out, formatApplyPlan(outcomes, prunes, dryRun))

	if !dryRun {
		newState.LastApplied = time.Now()
		if err := state.Save(statePath, newState); err != nil {
			return fmt.Errorf("apply: save state: %w", err)
		}
	}
	return nil
}

func saveState(path string, s state.State, dryRun bool) {
	if dryRun {
		return
	}
	s.LastApplied = time.Now()
	_ = state.Save(path, s) // best-effort partial save on the fail-fast path
}

func formatApplyPlan(outcomes []toolOutcome, prunes []pruneOutcome, dryRun bool) string {
	resources := 0
	for _, o := range outcomes {
		resources += len(o.Links) + len(o.Templates)
	}
	// Hooks are steps, not resources: they never count toward the summary
	// line, but a hook-only tool still deserves to be shown.
	if len(outcomes) == 0 && len(prunes) == 0 {
		return "Plan: 0 to create\n"
	}

	suffix := ""
	if dryRun {
		suffix = " (dry-run)"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Plan: %d to create, %d to prune\n\n", resources, len(prunes))
	for _, o := range outcomes {
		fmt.Fprintf(&b, "  %s\n", o.Tool)
		if o.PreApply != "" {
			fmt.Fprintf(&b, "    > run pre_apply%s   %s\n", suffix, o.PreApply)
		}
		for _, r := range o.Links {
			fmt.Fprintf(&b, "    + create symlink%s   %s -> %s\n", suffix, r.Target, r.Source)
		}
		for _, r := range o.Templates {
			fmt.Fprintf(&b, "    ~ render template%s  %s <- %s\n", suffix, r.Target, r.Source)
		}
		if o.PostApply != "" {
			fmt.Fprintf(&b, "    > run post_apply%s   %s\n", suffix, o.PostApply)
		}
		b.WriteString("\n")
	}
	if len(prunes) > 0 {
		b.WriteString("Prune:\n")
		for _, p := range prunes {
			b.WriteString(unlinkLine(p.Result, p.Type, p.BackupPath, suffix))
		}
	}
	return b.String()
}
