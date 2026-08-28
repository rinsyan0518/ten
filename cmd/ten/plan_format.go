package main

import (
	"fmt"
	"strings"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/plan"
)

// formatPlan renders a plan.Build plan for --dry-run: nothing has been
// executed, every line is a prediction. Layout and counting rules match
// formatApplyPlan so a dry-run reads like the receipt the real run will
// produce.
func formatPlan(pl plan.Plan) string {
	resources := 0
	for _, tp := range pl.Tools {
		for _, s := range tp.Links {
			if s.Action != plan.ActionNoop {
				resources++
			}
		}
		for _, s := range tp.Templates {
			if s.Action != plan.ActionNoop {
				resources++
			}
		}
	}
	prunes := 0
	for _, s := range pl.Prunes {
		if s.Action != plan.ActionSkip {
			prunes++
		}
	}
	if len(pl.Tools) == 0 && len(pl.Prunes) == 0 {
		return "Plan: 0 to create\n"
	}

	const suffix = " (dry-run)"
	var b strings.Builder
	fmt.Fprintf(&b, "Plan: %d to create, %d to prune\n\n", resources, prunes)
	for _, tp := range pl.Tools {
		fmt.Fprintf(&b, "  %s\n", tp.Tool)
		if tp.Before != "" {
			fmt.Fprintf(&b, "    > run before%s   %s\n", suffix, tp.Before)
		}
		for _, s := range tp.Links {
			switch s.Action {
			case plan.ActionNoop:
				fmt.Fprintf(&b, "    = symlink (up to date)   %s -> %s\n", s.Target, s.Source)
			case plan.ActionReplace:
				fmt.Fprintf(&b, "    ~ replace symlink%s   %s -> %s\n", suffix, s.Target, s.Source)
			default:
				fmt.Fprintf(&b, "    + create symlink%s   %s -> %s\n", suffix, s.Target, s.Source)
			}
		}
		for _, s := range tp.Templates {
			if s.Action == plan.ActionNoop {
				fmt.Fprintf(&b, "    = template (up to date)   %s <- %s\n", s.Target, s.Source)
				continue
			}
			fmt.Fprintf(&b, "    ~ render template%s  %s <- %s\n", suffix, s.Target, s.Source)
		}
		if tp.Once != "" {
			fmt.Fprintf(&b, "    > run once%s     %s\n", suffix, tp.Once)
		}
		if tp.After != "" {
			fmt.Fprintf(&b, "    > run after%s    %s\n", suffix, tp.After)
		}
		b.WriteString("\n")
	}
	if len(pl.Prunes) > 0 {
		b.WriteString("Prune:\n")
		for _, s := range pl.Prunes {
			b.WriteString(pruneStepLine(s, suffix))
		}
	}
	return b.String()
}

// formatApplyPlan renders the receipt of an executed apply run.
func formatApplyPlan(result apply.Result) string {
	resources := 0
	for _, o := range result.Outcomes {
		for _, r := range o.Links {
			if !r.Skipped {
				resources++
			}
		}
		for _, r := range o.Templates {
			if !r.Skipped {
				resources++
			}
		}
	}
	// Hooks are steps, not resources: they never count toward the summary
	// line, but a hook-only tool still deserves to be shown.
	if len(result.Outcomes) == 0 && len(result.Prunes) == 0 {
		return "Plan: 0 to create\n"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Plan: %d to create, %d to prune\n\n", resources, len(result.Prunes))
	for _, o := range result.Outcomes {
		fmt.Fprintf(&b, "  %s\n", o.Tool)
		if o.Before != "" {
			fmt.Fprintf(&b, "    > run before   %s\n", o.Before)
		}
		for _, r := range o.Links {
			if r.Skipped {
				fmt.Fprintf(&b, "    = symlink (up to date)   %s -> %s\n", r.Target, r.Source)
				continue
			}
			fmt.Fprintf(&b, "    + create symlink   %s -> %s\n", r.Target, r.Source)
		}
		for _, r := range o.Templates {
			if r.Skipped {
				fmt.Fprintf(&b, "    = template (up to date)   %s <- %s\n", r.Target, r.Source)
				continue
			}
			fmt.Fprintf(&b, "    ~ render template  %s <- %s\n", r.Target, r.Source)
		}
		if o.Once != "" {
			fmt.Fprintf(&b, "    > run once     %s\n", o.Once)
		}
		if o.After != "" {
			fmt.Fprintf(&b, "    > run after    %s\n", o.After)
		}
		b.WriteString("\n")
	}
	if len(result.Prunes) > 0 {
		b.WriteString("Prune:\n")
		for _, p := range result.Prunes {
			b.WriteString(unlinkLine(p.Result, p.Type, p.BackupPath, ""))
		}
	}
	return b.String()
}

// formatDestroyDryRun renders a plan.BuildDestroy plan for --dry-run.
func formatDestroyDryRun(dp plan.DestroyPlan) string {
	total := 0
	for _, tp := range dp.Tools {
		for _, s := range tp.Steps {
			if s.Action != plan.ActionSkip {
				total++
			}
		}
	}
	if len(dp.Tools) == 0 {
		return "Plan: 0 to destroy\n"
	}

	const suffix = " (dry-run)"
	var b strings.Builder
	fmt.Fprintf(&b, "Plan: %d to destroy\n\n", total)
	for _, tp := range dp.Tools {
		fmt.Fprintf(&b, "  %s\n", tp.Tool)
		for _, s := range tp.Steps {
			b.WriteString(pruneStepLine(s, suffix))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// formatDestroyPlan renders the receipt of an executed destroy run.
func formatDestroyPlan(result apply.DestroyResult) string {
	total := 0
	for _, o := range result.Outcomes {
		total += len(o.Entries)
	}
	if total == 0 {
		return "Plan: 0 to destroy\n"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Plan: %d to destroy\n\n", total)
	for _, o := range result.Outcomes {
		fmt.Fprintf(&b, "  %s\n", o.Tool)
		for _, e := range o.Entries {
			b.WriteString(unlinkLine(e.Result, e.Type, e.BackupPath, ""))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// pruneStepLine renders one planned removal/restore/skip, in the format
// shared by apply's Prune section and destroy's plan.
func pruneStepLine(s plan.PruneStep, suffix string) string {
	switch s.Action {
	case plan.ActionRestore:
		return fmt.Sprintf("    + restore backup%s    %s <- %s\n", suffix, s.Target, s.BackupPath)
	case plan.ActionSkip:
		return fmt.Sprintf("    ! skip    %s (%s)\n", s.Target, s.SkipReason)
	default:
		kind := s.Type
		if kind == "" {
			kind = "resource"
		}
		return fmt.Sprintf("    - remove %s%s    %s\n", kind, suffix, s.Target)
	}
}

// unlinkLine renders one resource that left ten's control, in the format
// shared by destroy's receipt and apply's Prune section: a restore names
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
