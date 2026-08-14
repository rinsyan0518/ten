package main

import (
	"fmt"
	"strings"

	"github.com/rinsyan0518/ten/internal/apply"
)

func formatApplyPlan(result apply.Result, dryRun bool) string {
	resources := 0
	for _, o := range result.Outcomes {
		resources += len(o.Links) + len(o.Templates)
	}
	// Hooks are steps, not resources: they never count toward the summary
	// line, but a hook-only tool still deserves to be shown.
	if len(result.Outcomes) == 0 && len(result.Prunes) == 0 {
		return "Plan: 0 to create\n"
	}

	suffix := ""
	if dryRun {
		suffix = " (dry-run)"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Plan: %d to create, %d to prune\n\n", resources, len(result.Prunes))
	for _, o := range result.Outcomes {
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
	if len(result.Prunes) > 0 {
		b.WriteString("Prune:\n")
		for _, p := range result.Prunes {
			b.WriteString(unlinkLine(p.Result, p.Type, p.BackupPath, suffix))
		}
	}
	return b.String()
}

func formatDestroyPlan(result apply.DestroyResult, dryRun bool) string {
	total := 0
	for _, o := range result.Outcomes {
		total += len(o.Entries)
	}
	if total == 0 {
		return "Plan: 0 to destroy\n"
	}

	suffix := ""
	if dryRun {
		suffix = " (dry-run)"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Plan: %d to destroy\n\n", total)
	for _, o := range result.Outcomes {
		fmt.Fprintf(&b, "  %s\n", o.Tool)
		for _, e := range o.Entries {
			b.WriteString(unlinkLine(e.Result, e.Type, e.BackupPath, suffix))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// unlinkLine renders one resource leaving ten's control, in the format
// shared by destroy's plan and apply's Prune section: a restore names
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
