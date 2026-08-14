package config

import "fmt"

// Merge combines the base repo config, an optional profile-specific repo
// config, and the local config into a single Merged configuration.
//
// [tools.*] definitions are combined with whole-tool replace, applied in
// order base -> profile -> local (a later layer's definition of a tool
// name fully replaces the earlier one).
//
// enabled_tools is resolved as the union of every EnabledTools list
// declared (non-nil) across the three layers. If none of the three
// layers declare enabled_tools, every defined tool is enabled (backward
// compatible default).
func Merge(base Repo, profile *Repo, local Local) (Merged, error) {
	tools := make(map[string]Tool)
	for name, t := range base.Tools {
		tools[name] = t
	}
	if profile != nil {
		for name, t := range profile.Tools {
			tools[name] = t
		}
	}
	for name, t := range local.Tools {
		tools[name] = t
	}

	declared := base.EnabledTools != nil || local.EnabledTools != nil ||
		(profile != nil && profile.EnabledTools != nil)

	enabled := make(map[string]bool)
	if !declared {
		for name := range tools {
			enabled[name] = true
		}
	} else {
		addAll := func(list []string) error {
			for _, name := range list {
				if _, ok := tools[name]; !ok {
					return fmt.Errorf("config: enabled_tools references undefined tool %q", name)
				}
				enabled[name] = true
			}
			return nil
		}
		if err := addAll(base.EnabledTools); err != nil {
			return Merged{}, err
		}
		if profile != nil {
			if err := addAll(profile.EnabledTools); err != nil {
				return Merged{}, err
			}
		}
		if err := addAll(local.EnabledTools); err != nil {
			return Merged{}, err
		}
	}

	return Merged{
		DotfilesRoot: local.Core.DotfilesRoot,
		Vars:         local.Vars,
		Tools:        tools,
		Enabled:      enabled,
	}, nil
}
