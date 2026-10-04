package config

// Layer is one config file in the merge order, tagged with the root its
// tools' links/templates sources (and hook working directory) resolve
// against. base/profile/ten.local.toml always carry the same
// dotfilesRoot; an external root's layer carries its own path.
type Layer struct {
	Root string
	File File
}

// mergeTool overlays override onto base, replacing only the fields
// override actually sets. A field counts as "set" when its value is
// distinguishable from "not present in this layer's TOML": a non-nil
// pointer for Enabled, a non-nil map/slice for Links/Templates/DependsOn
// (TOML decoding leaves an omitted key as the Go zero value, nil, while
// an explicitly empty table/array like `links = {}` decodes to a non-nil
// empty collection), and a non-empty string for Before/Once/After.
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
	if override.Before != "" {
		merged.Before = override.Before
	}
	if override.Once != "" {
		merged.Once = override.Once
	}
	if override.After != "" {
		merged.After = override.After
	}
	return merged
}

// Merge combines an ordered list of config layers into a single Merged
// configuration. [tools.*] is combined with layered per-field override,
// applied in the given order: a later layer's value for a given field
// fully replaces the earlier one only when that field is actually set in
// the later layer (see mergeTool); fields left unset in a later layer
// keep the value from the earliest layer that set them. Tool names are
// the union of every name declared across all layers.
//
// vars follows the same layering but merges per variable key rather than
// per tool: a variable declared in a later layer overrides only that one
// key, leaving variables declared solely in earlier layers untouched.
//
// Each tool's Enabled field follows the same per-field rule as the rest
// of Tool: nil means "not set in this layer," and a tool whose Enabled
// is nil after every layer is folded defaults to enabled (true).
//
// LinksRoot/TemplatesRoot/HookRoot are tracked per tool, independently
// per field group: a layer that sets Links for a tool moves that tool's
// LinksRoot to the layer's Root, regardless of whether the same layer
// touched Templates or the hook fields. This mirrors mergeTool's own
// per-field "did this layer actually set it" checks, so a layer that
// only flips a tool's Enabled never drags its other fields' root along
// with it.
func Merge(layers []Layer) (Merged, error) {
	tools := make(map[string]Tool)
	linksRoot := make(map[string]string)
	templatesRoot := make(map[string]string)
	hookRoot := make(map[string]string)
	vars := make(map[string]string)

	for _, l := range layers {
		for name, t := range l.File.Tools {
			tools[name] = mergeTool(tools[name], t)
			if t.Links != nil {
				linksRoot[name] = l.Root
			}
			if t.Templates != nil {
				templatesRoot[name] = l.Root
			}
			if t.Before != "" || t.Once != "" || t.After != "" {
				hookRoot[name] = l.Root
			}
		}
		for k, v := range l.File.Vars {
			vars[k] = v
		}
	}

	enabled := make(map[string]bool)
	for name, t := range tools {
		enabled[name] = t.Enabled == nil || *t.Enabled
	}

	return Merged{
		Vars:          vars,
		Tools:         tools,
		Enabled:       enabled,
		LinksRoot:     linksRoot,
		TemplatesRoot: templatesRoot,
		HookRoot:      hookRoot,
	}, nil
}
