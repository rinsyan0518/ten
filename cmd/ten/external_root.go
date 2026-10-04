package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/rinsyan0518/ten/internal/pathresolve"
	"github.com/rinsyan0518/ten/internal/state"
	"github.com/spf13/cobra"
)

var externalRootNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// newExternalRootCmd builds the `ten root` command group. It is named
// newExternalRootCmd (not newRootCmd) because cmd/ten/root.go already
// defines newRootCmd for the CLI's top-level `ten` command — the clash
// is in Go symbol names only; the CLI command text is "root" either way.
func newExternalRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "root",
		Short: "Manage additional named config roots (e.g. a private company repo)",
	}
	cmd.AddCommand(newRootAddCmd())
	cmd.AddCommand(newRootRemoveCmd())
	cmd.AddCommand(newRootListCmd())
	return cmd
}

func newRootAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <path>",
		Args:  cobra.ExactArgs(2),
		Short: "Register (or update) a named external config root",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRootAdd(cmd, args[0], args[1])
		},
	}
}

func runRootAdd(cmd *cobra.Command, name, path string) error {
	if !externalRootNamePattern.MatchString(name) {
		return fmt.Errorf("root add: invalid name %q (must match %s)", name, externalRootNamePattern.String())
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("root add: resolve home dir: %w", err)
	}
	env := pathresolve.FromOS(home)
	st, statePath, err := loadBootstrap(env)
	if err != nil {
		return fmt.Errorf("root add: %w", err)
	}

	path = pathresolve.ExpandHome(path, home)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("root add: resolve path %s: %w", path, err)
	}
	if info, statErr := os.Stat(absPath); statErr != nil {
		return fmt.Errorf("root add: %s is not an existing directory: %w", absPath, statErr)
	} else if !info.IsDir() {
		return fmt.Errorf("root add: %s is not an existing directory", absPath)
	}
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		absPath = resolved
	}

	updated := false
	for i, r := range st.ExternalRoots {
		if r.Name == name {
			st.ExternalRoots[i].Path = absPath
			updated = true
			break
		}
	}
	if !updated {
		st.ExternalRoots = append(st.ExternalRoots, state.ExternalRoot{Name: name, Path: absPath})
	}

	if err := state.Save(statePath, st); err != nil {
		return fmt.Errorf("root add: save state: %w", err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Registered external root %q at %s\n", name, absPath)
	return nil
}

func newRootRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Remove a registered external config root",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRootRemove(cmd, args[0])
		},
	}
}

func runRootRemove(cmd *cobra.Command, name string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("root remove: resolve home dir: %w", err)
	}
	env := pathresolve.FromOS(home)
	st, statePath, err := loadBootstrap(env)
	if err != nil {
		return fmt.Errorf("root remove: %w", err)
	}

	index := -1
	for i, r := range st.ExternalRoots {
		if r.Name == name {
			index = i
			break
		}
	}
	if index == -1 {
		return fmt.Errorf("root remove: no external root named %q is registered", name)
	}
	st.ExternalRoots = append(st.ExternalRoots[:index], st.ExternalRoots[index+1:]...)

	if err := state.Save(statePath, st); err != nil {
		return fmt.Errorf("root remove: save state: %w", err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removed external root %q\n", name)
	return nil
}

func newRootListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Args:  cobra.NoArgs,
		Short: "List registered external config roots in merge order",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRootList(cmd)
		},
	}
}

func runRootList(cmd *cobra.Command) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("root list: resolve home dir: %w", err)
	}
	env := pathresolve.FromOS(home)
	st, _, err := loadBootstrap(env)
	if err != nil {
		return fmt.Errorf("root list: %w", err)
	}
	if len(st.ExternalRoots) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No external roots registered.")
		return nil
	}
	for _, r := range st.ExternalRoots {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", r.Name, r.Path)
	}
	return nil
}
