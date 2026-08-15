package config

import "fmt"

// Merge combines the base repo config, an optional profile-specific repo
// config, and an optional local config into a single Merged
// configuration.
//
// [tools.*] and vars are both combined with layered key-level override,
// applied in order base -> profile -> local (a later layer's value for a
// given key fully replaces the earlier one; [tools.*] overrides by tool
// name, vars overrides by var name).
//
// enabled_tools is resolved as the union of every EnabledTools list
// declared (non-nil) across the three layers. If none of the three
// layers declare enabled_tools, every defined tool is enabled (backward
// compatible default).
func Merge(base File, profile *File, local *File) (Merged, error) {
	tools := make(map[string]Tool)
	for name, t := range base.Tools {
		tools[name] = t
	}
	if profile != nil {
		for name, t := range profile.Tools {
			tools[name] = t
		}
	}
	if local != nil {
		for name, t := range local.Tools {
			tools[name] = t
		}
	}

	vars := make(map[string]string)
	for k, v := range base.Vars {
		vars[k] = v
	}
	if profile != nil {
		for k, v := range profile.Vars {
			vars[k] = v
		}
	}
	if local != nil {
		for k, v := range local.Vars {
			vars[k] = v
		}
	}

	var localEnabledTools []string
	if local != nil {
		localEnabledTools = local.EnabledTools
	}
	declared := base.EnabledTools != nil || localEnabledTools != nil ||
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
		if err := addAll(localEnabledTools); err != nil {
			return Merged{}, err
		}
	}

	return Merged{
		Vars:    vars,
		Tools:   tools,
		Enabled: enabled,
	}, nil
}
