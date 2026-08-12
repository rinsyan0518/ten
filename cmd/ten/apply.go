package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/graph"
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

	merged, err := loadMerged(home)
	if err != nil {
		return fmt.Errorf("apply: load config: %w", err)
	}
	order, err := graph.Sort(merged)
	if err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	backupDir := filepath.Join(home, ".ten_backup")

	var outcomes []toolOutcome
	for _, name := range order {
		tool := merged.Tools[name]
		linkKeys := make([]string, 0, len(tool.Links))
		for k := range tool.Links {
			linkKeys = append(linkKeys, k)
		}
		sort.Strings(linkKeys)

		var links []apply.LinkResult
		for _, key := range linkKeys {
			target, err := resolveKey(key, home)
			if err != nil {
				return fmt.Errorf("apply: tool %s: %w", name, err)
			}
			source := filepath.Join(merged.DotfilesRoot, tool.Links[key])

			result, err := apply.Link(target, source, backupDir, dryRun)
			if err != nil {
				return fmt.Errorf("apply: tool %s: %w", name, err)
			}
			if !result.Skipped {
				links = append(links, result)
			}
		}
		templateKeys := make([]string, 0, len(tool.Templates))
		for k := range tool.Templates {
			templateKeys = append(templateKeys, k)
		}
		sort.Strings(templateKeys)

		var templates []apply.TemplateResult
		for _, key := range templateKeys {
			target, err := resolveKey(key, home)
			if err != nil {
				return fmt.Errorf("apply: tool %s: %w", name, err)
			}
			source := filepath.Join(merged.DotfilesRoot, tool.Templates[key])

			result, err := apply.RenderTemplate(target, source, merged.Vars, backupDir, false, dryRun)
			if err != nil {
				return fmt.Errorf("apply: tool %s: %w", name, err)
			}
			templates = append(templates, result)
		}
		if len(links) > 0 || len(templates) > 0 {
			outcomes = append(outcomes, toolOutcome{Tool: name, Links: links, Templates: templates})
		}
	}

	fmt.Fprint(cmd.OutOrStdout(), formatApplyPlan(outcomes, dryRun))
	return nil
}

func formatApplyPlan(outcomes []toolOutcome, dryRun bool) string {
	total := 0
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
	fmt.Fprintf(&b, "Plan: %d to create\n\n", total)
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
	return b.String()
}
