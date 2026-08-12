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
	Tool  string
	Links []apply.LinkResult
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
		if len(links) > 0 {
			outcomes = append(outcomes, toolOutcome{Tool: name, Links: links})
		}
	}

	fmt.Fprint(cmd.OutOrStdout(), formatApplyPlan(outcomes, dryRun))
	return nil
}

func formatApplyPlan(outcomes []toolOutcome, dryRun bool) string {
	total := 0
	for _, o := range outcomes {
		total += len(o.Links)
	}
	if total == 0 {
		return "Plan: 0 to create\n"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Plan: %d to create\n\n", total)
	for _, o := range outcomes {
		fmt.Fprintf(&b, "  %s\n", o.Tool)
		for _, r := range o.Links {
			verb := "+ create symlink"
			if dryRun {
				verb = "+ create symlink (dry-run)"
			}
			fmt.Fprintf(&b, "    %s   %s -> %s\n", verb, r.Target, r.Source)
		}
		b.WriteString("\n")
	}
	return b.String()
}
