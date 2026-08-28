package plan

import (
	"sort"

	"github.com/rinsyan0518/ten/internal/state"
)

// DestroyToolPlan groups one tool's planned removals.
type DestroyToolPlan struct {
	Tool  string
	Steps []PruneStep
}

// DestroyPlan is everything one destroy run intends to do, derived
// entirely from the state file plus a read-only look at the disk — no
// config is consulted, so destroy keeps working even when the dotfiles
// repo is gone.
type DestroyPlan struct {
	Tools []DestroyToolPlan
}

// BuildDestroy plans the removal of every resource recorded in current,
// in a deterministic order (tool name, then target name within a tool).
// Each step's action — remove, restore, or skip with a reason — is
// decided here against what is actually on disk, so a dry-run prints
// exactly what a real run would do. Like Build, it only reads.
func BuildDestroy(current state.State, ins Inspector) (DestroyPlan, error) {
	byTool := make(map[string][]string)
	for target, res := range current.ManagedResources {
		byTool[res.Tool] = append(byTool[res.Tool], target)
	}
	tools := make([]string, 0, len(byTool))
	for tool := range byTool {
		sort.Strings(byTool[tool])
		tools = append(tools, tool)
	}
	sort.Strings(tools)

	var dp DestroyPlan
	for _, tool := range tools {
		tp := DestroyToolPlan{Tool: tool}
		for _, target := range byTool[tool] {
			step, err := planPrune(target, current.ManagedResources[target], ins)
			if err != nil {
				return DestroyPlan{}, err
			}
			tp.Steps = append(tp.Steps, step)
		}
		dp.Tools = append(dp.Tools, tp)
	}
	return dp, nil
}
