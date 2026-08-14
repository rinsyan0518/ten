package plan

import "github.com/rinsyan0518/ten/internal/state"

// Prune returns the target paths present in current's ManagedResources
// but absent from desiredTargets (the set of target paths this apply run
// intends to manage). Order is not guaranteed.
func Prune(current state.State, desiredTargets map[string]bool) []string {
	var stale []string
	for target := range current.ManagedResources {
		if !desiredTargets[target] {
			stale = append(stale, target)
		}
	}
	return stale
}
