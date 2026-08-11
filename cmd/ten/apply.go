package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/rinsyan0518/ten/internal/apply"
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

func runApply(cmd *cobra.Command, dryRun bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("apply: resolve home dir: %w", err)
	}

	merged, err := loadMerged(home)
	if err != nil {
		return fmt.Errorf("apply: load config: %w", err)
	}
	backupDir := filepath.Join(home, ".ten_backup")

	names := make([]string, 0, len(merged.Tools))
	for name := range merged.Tools {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
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
			source := filepath.Join(merged.DotfilesRoot, tool.Links[key])

			result, err := apply.Link(target, source, backupDir, dryRun)
			if err != nil {
				return fmt.Errorf("apply: tool %s: %w", name, err)
			}
			printLinkResult(cmd, name, result, dryRun)
		}
	}
	return nil
}

func printLinkResult(cmd *cobra.Command, tool string, r apply.LinkResult, dryRun bool) {
	if r.Skipped {
		return
	}
	verb := "+ create symlink"
	if dryRun {
		verb = "+ create symlink (dry-run)"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  %s\n    %s   %s -> %s\n", tool, verb, r.Target, r.Source)
}
