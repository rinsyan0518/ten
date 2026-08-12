package plan_test

import (
	"testing"

	"github.com/rinsyan0518/ten/internal/config"
	"github.com/rinsyan0518/ten/internal/plan"
)

func TestDestroyOrder_ReversesApplyOrder(t *testing.T) {
	tools := map[string]config.Tool{
		"git":      {},
		"git-work": {DependsOn: []string{"git"}},
	}
	managed := map[string]bool{"git": true, "git-work": true}

	order, err := plan.DestroyOrder(tools, managed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pos := map[string]int{}
	for i, name := range order {
		pos[name] = i
	}
	if pos["git-work"] >= pos["git"] {
		t.Fatalf("expected git-work destroyed before git, got order %v", order)
	}
}

func TestDestroyOrder_DiscardsNaiveAlphabeticalSort(t *testing.T) {
	// Use tool names where topological and alphabetical order diverge.
	// Topological order (apply): [zzz, aaa] (zzz has no deps, aaa depends on zzz)
	// Correct destroy order (reverse): [aaa, zzz]
	// Naive alphabetical-descending: [zzz, aaa] (wrong!)
	tools := map[string]config.Tool{
		"zzz": {},
		"aaa": {DependsOn: []string{"zzz"}},
	}
	managed := map[string]bool{"zzz": true, "aaa": true}

	order, err := plan.DestroyOrder(tools, managed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pos := map[string]int{}
	for i, name := range order {
		pos[name] = i
	}
	if pos["aaa"] >= pos["zzz"] {
		t.Fatalf("expected aaa destroyed before zzz (correct topological reversal), got order %v", order)
	}
}

func TestDestroyOrder_ErrorsWhenToolDefinitionMissing(t *testing.T) {
	managed := map[string]bool{"ghost-tool": true}
	if _, err := plan.DestroyOrder(map[string]config.Tool{}, managed); err == nil {
		t.Fatalf("expected error when a managed tool has no current definition")
	}
}
