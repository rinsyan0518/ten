package plan

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/rinsyan0518/ten/internal/config"
	"github.com/rinsyan0518/ten/internal/pathresolve"
)

// Target is one desired filesystem resource: a symlink or rendered
// template that a tool wants to exist at Target's path.
type Target struct {
	Tool   string
	Kind   string // "symlink" | "template"
	Target string
	Source string
}

// Desired resolves every desired link/template target for merged's tools,
// walked in order (the DAG order from graph.Sort). Per-tool link/template
// keys are sorted for deterministic output.
//
// Two entries resolving to the same target path are an error, whether
// they come from different tools or from one tool's links and templates:
// left undetected, the second claimant would treat the first's resource
// as a foreign file and back it up and replace it on every apply,
// flip-flopping forever.
func Desired(merged config.Merged, order []string, env pathresolve.Env) ([]Target, error) {
	var desired []Target
	claimed := make(map[string]Target)
	claim := func(t Target) error {
		if prev, ok := claimed[t.Target]; ok {
			return fmt.Errorf("target %s is claimed twice: by tool %s (%s %s) and tool %s (%s %s)",
				t.Target, prev.Tool, prev.Kind, prev.Source, t.Tool, t.Kind, t.Source)
		}
		claimed[t.Target] = t
		return nil
	}
	for _, name := range order {
		tool := merged.Tools[name]

		linkKeys := make([]string, 0, len(tool.Links))
		for k := range tool.Links {
			linkKeys = append(linkKeys, k)
		}
		sort.Strings(linkKeys)
		for _, key := range linkKeys {
			target, err := pathresolve.Resolve(env, key)
			if err != nil {
				return nil, fmt.Errorf("tool %s: %w", name, err)
			}
			t := Target{Tool: name, Kind: "symlink", Target: target, Source: filepath.Join(merged.DotfilesRoot, tool.Links[key])}
			if err := claim(t); err != nil {
				return nil, err
			}
			desired = append(desired, t)
		}

		templateKeys := make([]string, 0, len(tool.Templates))
		for k := range tool.Templates {
			templateKeys = append(templateKeys, k)
		}
		sort.Strings(templateKeys)
		for _, key := range templateKeys {
			target, err := pathresolve.Resolve(env, key)
			if err != nil {
				return nil, fmt.Errorf("tool %s: %w", name, err)
			}
			t := Target{Tool: name, Kind: "template", Target: target, Source: filepath.Join(merged.DotfilesRoot, tool.Templates[key])}
			if err := claim(t); err != nil {
				return nil, err
			}
			desired = append(desired, t)
		}
	}
	return desired, nil
}
