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
	Links     []apply.LinkResult
	Templates []apply.TemplateResult
}

func runApply(cmd *cobra.Command, dryRun bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("apply: resolve home dir: %w", err)
	}
	statePath := filepath.Join(home, ".config", "ten", "ten.state.json")

	merged, err := loadMerged(home)
	if err != nil {
		return fmt.Errorf("apply: load config: %w", err)
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

	// Seed the new state from whatever is still desired, so a resource
	// this run never reaches (because an earlier tool failed first) keeps
	// its prior record instead of being silently dropped from tracking.
	newState := state.State{ManagedResources: make(map[string]state.Resource, len(desiredSet))}
	for target, res := range current.ManagedResources {
		if desiredSet[target] {
			newState.ManagedResources[target] = res
		}
	}

	for _, target := range pruneTargets {
		if !dryRun {
			if err := os.RemoveAll(target); err != nil {
				return fmt.Errorf("apply: prune %s: %w", target, err)
			}
		}
	}

	byTool := make(map[string][]desiredTarget)
	var toolOrder []string
	for _, d := range desired {
		if _, seen := byTool[d.tool]; !seen {
			toolOrder = append(toolOrder, d.tool)
		}
		byTool[d.tool] = append(byTool[d.tool], d)
	}

	var outcomes []toolOutcome
	for _, name := range toolOrder {
		var links []apply.LinkResult
		var templates []apply.TemplateResult
		for _, d := range byTool[name] {
			switch d.kind {
			case "symlink":
				result, err := apply.Link(d.target, d.source, backupDir, dryRun)
				if err != nil {
					saveState(statePath, newState, dryRun)
					return fmt.Errorf("apply: tool %s: %w", name, err)
				}
				if !dryRun {
					newState.ManagedResources[d.target] = state.Resource{Tool: name, Type: "symlink", Source: d.source}
				}
				if !result.Skipped {
					links = append(links, result)
				}
			case "template":
				_, alreadyManaged := current.ManagedResources[d.target]
				result, err := apply.RenderTemplate(d.target, d.source, merged.Vars, backupDir, alreadyManaged, dryRun)
				if err != nil {
					saveState(statePath, newState, dryRun)
					return fmt.Errorf("apply: tool %s: %w", name, err)
				}
				if !dryRun {
					newState.ManagedResources[d.target] = state.Resource{Tool: name, Type: "template", Source: d.source}
				}
				templates = append(templates, result)
			}
		}
		if len(links) > 0 || len(templates) > 0 {
			outcomes = append(outcomes, toolOutcome{Tool: name, Links: links, Templates: templates})
		}
	}

	fmt.Fprint(cmd.OutOrStdout(), formatApplyPlan(outcomes, pruneTargets, dryRun))

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

func formatApplyPlan(outcomes []toolOutcome, pruneTargets []string, dryRun bool) string {
	total := len(pruneTargets)
	for _, o := range outcomes {
		total += len(o.Links) + len(o.Templates)
	}
	if total == 0 {
		return "Plan: 0 to create\n"
	}

	suffix := ""
	if dryRun {
		suffix = " (dry-run)"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Plan: %d to create, %d to prune\n\n", total-len(pruneTargets), len(pruneTargets))
	for _, o := range outcomes {
		fmt.Fprintf(&b, "  %s\n", o.Tool)
		for _, r := range o.Links {
			fmt.Fprintf(&b, "    + create symlink%s   %s -> %s\n", suffix, r.Target, r.Source)
		}
		for _, r := range o.Templates {
			fmt.Fprintf(&b, "    ~ render template%s  %s <- %s\n", suffix, r.Target, r.Source)
		}
		b.WriteString("\n")
	}
	if len(pruneTargets) > 0 {
		b.WriteString("Prune:\n")
		for _, target := range pruneTargets {
			fmt.Fprintf(&b, "    - remove%s   %s\n", suffix, target)
		}
	}
	return b.String()
}
