package plan

import (
	"fmt"

	"github.com/rinsyan0518/ten/internal/config"
	"github.com/rinsyan0518/ten/internal/graph"
)

// DestroyOrder returns tool names in reverse dependency order (the mirror
// of Apply's order) for the given managed tool set: a tool is destroyed
// after everything that depends on it.
//
// allTools must contain a definition for every name in managedTools (it
// is the currently loaded config's full tool map); if a managed tool's
// definition can't be found — e.g. destroy is run under a different
// profile than the one used to apply — an error is returned.
func DestroyOrder(allTools map[string]config.Tool, managedTools map[string]bool) ([]string, error) {
	for name := range managedTools {
		if _, ok := allTools[name]; !ok {
			return nil, fmt.Errorf("plan: managed tool %q has no definition in the currently loaded config", name)
		}
	}

	forward, err := graph.Sort(config.Merged{Tools: allTools, Enabled: managedTools})
	if err != nil {
		return nil, err
	}
	reversed := make([]string, len(forward))
	for i, name := range forward {
		reversed[len(forward)-1-i] = name
	}
	return reversed, nil
}
