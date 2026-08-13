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

	// Order over every defined tool, not just the managed subset: a managed
	// tool may validly depend on a tool that currently owns no resources
	// (all of its links were pruned, or it only defines hooks), and sorting
	// the subset alone would reject that as an unenabled dependency.
	all := make(map[string]bool, len(allTools))
	for name := range allTools {
		all[name] = true
	}
	forward, err := graph.Sort(config.Merged{Tools: allTools, Enabled: all})
	if err != nil {
		return nil, err
	}

	order := make([]string, 0, len(managedTools))
	for i := len(forward) - 1; i >= 0; i-- {
		if managedTools[forward[i]] {
			order = append(order, forward[i])
		}
	}
	return order, nil
}
