package config

// mergeTool overlays override onto base, replacing only the fields
// override actually sets. A field counts as "set" when its value is
// distinguishable from "not present in this layer's TOML": a non-nil
// pointer for Enabled, a non-nil map/slice for Links/Templates/DependsOn
// (TOML decoding leaves an omitted key as the Go zero value, nil, while
// an explicitly empty table/array like `links = {}` decodes to a non-nil
// empty collection), and a non-empty string for PreApply/PostApply.
func mergeTool(base, override Tool) Tool {
	merged := base
	if override.Enabled != nil {
		merged.Enabled = override.Enabled
	}
	if override.Links != nil {
		merged.Links = override.Links
	}
	if override.Templates != nil {
		merged.Templates = override.Templates
	}
	if override.DependsOn != nil {
		merged.DependsOn = override.DependsOn
	}
	if override.PreApply != "" {
		merged.PreApply = override.PreApply
	}
	if override.PostApply != "" {
		merged.PostApply = override.PostApply
	}
	return merged
}

// Merge combines the base repo config, an optional profile-specific repo
// config, and an optional local config into a single Merged
// configuration.
//
// [tools.*] is combined with layered per-field override, applied in
// order base -> profile -> local: a later layer's value for a given
// field fully replaces the earlier one only when that field is actually
// set in the later layer (see mergeTool); fields left unset in a later
// layer keep the value from the earliest layer that set them. Tool names
// are the union of every name declared across the three layers.
//
// vars follows the same base -> profile -> local layering but merges per
// variable key rather than per tool: a variable declared in a later
// layer overrides only that one key, leaving variables declared solely
// in earlier layers untouched.
//
// Each tool's Enabled field follows the same per-field rule as the rest
// of Tool: nil means "not set in this layer," and a tool whose Enabled
// is nil after all three layers are folded defaults to enabled (true).
func Merge(base File, profile *File, local *File) (Merged, error) {
	tools := make(map[string]Tool)
	for name, t := range base.Tools {
		tools[name] = t
	}
	overlay := func(layer *File) {
		if layer == nil {
			return
		}
		for name, t := range layer.Tools {
			tools[name] = mergeTool(tools[name], t)
		}
	}
	overlay(profile)
	overlay(local)

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

	enabled := make(map[string]bool)
	for name, t := range tools {
		enabled[name] = t.Enabled == nil || *t.Enabled
	}

	return Merged{
		Vars:    vars,
		Tools:   tools,
		Enabled: enabled,
	}, nil
}
