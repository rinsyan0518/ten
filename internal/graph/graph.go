package graph

import (
	"fmt"
	"sort"

	"github.com/rinsyan0518/ten/internal/config"
)

// Sort computes a dependency-respecting execution order for the enabled
// tools in merged, using each tool's DependsOn list.
func Sort(merged config.Merged) ([]string, error) {
	for name := range merged.Enabled {
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

	inDegree := make(map[string]int, len(merged.Enabled))
	dependents := make(map[string][]string, len(merged.Enabled))
	for name := range merged.Enabled {
		inDegree[name] = 0
	}
	for name := range merged.Enabled {
		for _, dep := range merged.Tools[name].DependsOn {
			dependents[dep] = append(dependents[dep], name)
			inDegree[name]++
		}
	}

	var queue []string
	for name := range merged.Enabled {
		if inDegree[name] == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	order := make([]string, 0, len(merged.Enabled))
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

	if len(order) != len(merged.Enabled) {
		return nil, fmt.Errorf("graph: dependency cycle detected among enabled tools")
	}
	return order, nil
}
