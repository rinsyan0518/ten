package main

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "ten",
		Short: "ten is an idempotent dotfiles manager",
	}
	root.AddCommand(newApplyCmd())
	return root
}
