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
func Desired(merged config.Merged, order []string, home string) ([]Target, error) {
	var desired []Target
	for _, name := range order {
		tool := merged.Tools[name]

		linkKeys := make([]string, 0, len(tool.Links))
		for k := range tool.Links {
			linkKeys = append(linkKeys, k)
		}
		sort.Strings(linkKeys)
		for _, key := range linkKeys {
			target, err := pathresolve.ResolveKey(home, key)
			if err != nil {
				return nil, fmt.Errorf("plan: tool %s: %w", name, err)
			}
			desired = append(desired, Target{Tool: name, Kind: "symlink", Target: target, Source: filepath.Join(merged.DotfilesRoot, tool.Links[key])})
		}

		templateKeys := make([]string, 0, len(tool.Templates))
		for k := range tool.Templates {
			templateKeys = append(templateKeys, k)
		}
		sort.Strings(templateKeys)
		for _, key := range templateKeys {
			target, err := pathresolve.ResolveKey(home, key)
			if err != nil {
				return nil, fmt.Errorf("plan: tool %s: %w", name, err)
			}
			desired = append(desired, Target{Tool: name, Kind: "template", Target: target, Source: filepath.Join(merged.DotfilesRoot, tool.Templates[key])})
		}
	}
	return desired, nil
}
