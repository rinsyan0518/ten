package graph

import (
	"fmt"
	"sort"

	"github.com/rinsyan0518/ten/internal/config"
)

// Sort computes a dependency-respecting execution order for the enabled
// tools in merged, using each tool's DependsOn list.
func Sort(merged config.Merged) ([]string, error) {
	// Merged.Enabled carries an explicit entry for every defined tool,
	// including disabled ones, so its key set is not the enabled set —
	// filter on the value here rather than ranging the map directly.
	var enabledTools []string
	for name, enabled := range merged.Enabled {
		if enabled {
			enabledTools = append(enabledTools, name)
		}
	}

	for _, name := range enabledTools {
		tool, ok := merged.Tools[name]
		if !ok {
			return nil, fmt.Errorf("graph: enabled tool %q has no definition", name)
		}
		for _, dep := range tool.DependsOn {
			if _, exists := merged.Tools[dep]; !exists {
				return nil, fmt.Errorf("graph: tool %q depends on %q, which is not defined", name, dep)
			}
			if !merged.Enabled[dep] {
				return nil, fmt.Errorf("graph: tool %q depends on %q, which is not enabled", name, dep)
			}
		}
	}

	inDegree := make(map[string]int, len(enabledTools))
	dependents := make(map[string][]string, len(enabledTools))
	for _, name := range enabledTools {
		inDegree[name] = 0
	}
	for _, name := range enabledTools {
		for _, dep := range merged.Tools[name].DependsOn {
			dependents[dep] = append(dependents[dep], name)
			inDegree[name]++
		}
	}

	var queue []string
	for _, name := range enabledTools {
		if inDegree[name] == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	order := make([]string, 0, len(enabledTools))
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		order = append(order, n)

		next := append([]string{}, dependents[n]...)
		sort.Strings(next)
		for _, m := range next {
			inDegree[m]--
			if inDegree[m] == 0 {
				queue = append(queue, m)
				sort.Strings(queue)
			}
		}
	}

	if len(order) != len(enabledTools) {
		return nil, fmt.Errorf("graph: dependency cycle detected among enabled tools")
	}
	return order, nil
}
