package graph_test

import (
	"testing"

	"github.com/rinsyan0518/ten/internal/config"
	"github.com/rinsyan0518/ten/internal/graph"
)

func TestSort_OrdersByDependency(t *testing.T) {
	merged := config.Merged{
		Tools: map[string]config.Tool{
			"git":      {},
			"git-work": {DependsOn: []string{"git"}},
			"nvim":     {},
		},
		Enabled: map[string]bool{"git": true, "git-work": true, "nvim": true},
	}

	order, err := graph.Sort(merged)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 tools in order, got %v", order)
	}
	pos := map[string]int{}
	for i, name := range order {
		pos[name] = i
	}
	if pos["git"] >= pos["git-work"] {
		t.Fatalf("expected git before git-work, got order %v", order)
	}
}

func TestSort_ErrorsOnUnenabledDependency(t *testing.T) {
	merged := config.Merged{
		Tools:   map[string]config.Tool{"git": {}, "nvim": {DependsOn: []string{"git"}}},
		Enabled: map[string]bool{"nvim": true},
	}
	if _, err := graph.Sort(merged); err == nil {
		t.Fatalf("expected error for dependency on unenabled tool")
	}
}

func TestSort_ErrorsOnUndefinedDependency(t *testing.T) {
	merged := config.Merged{
		Tools:   map[string]config.Tool{"nvim": {DependsOn: []string{"ghost"}}},
		Enabled: map[string]bool{"nvim": true},
	}
	if _, err := graph.Sort(merged); err == nil {
		t.Fatalf("expected error for dependency on undefined tool")
	}
}

func TestSort_ErrorsOnCycle(t *testing.T) {
	merged := config.Merged{
		Tools: map[string]config.Tool{
			"a": {DependsOn: []string{"b"}},
			"b": {DependsOn: []string{"a"}},
		},
		Enabled: map[string]bool{"a": true, "b": true},
	}
	if _, err := graph.Sort(merged); err == nil {
		t.Fatalf("expected error for cycle")
	}
}

func TestSort_ExcludesExplicitlyDisabledTool(t *testing.T) {
	merged := config.Merged{
		Tools:   map[string]config.Tool{"git": {}, "nvim": {}},
		Enabled: map[string]bool{"git": true, "nvim": false},
	}

	order, err := graph.Sort(merged)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 1 || order[0] != "git" {
		t.Fatalf("expected only git in order, got %v", order)
	}
}
