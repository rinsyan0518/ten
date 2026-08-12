package plan_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/rinsyan0518/ten/internal/plan"
	"github.com/rinsyan0518/ten/internal/state"
)

func TestPrune_ReturnsResourcesNotInDesired(t *testing.T) {
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.gitconfig":     {Tool: "git"},
		"/home/taro/.config/nvim":   {Tool: "nvim"},
		"/home/taro/.config/old":    {Tool: "removed-tool"},
	}}
	desired := map[string]bool{
		"/home/taro/.gitconfig":   true,
		"/home/taro/.config/nvim": true,
	}

	got := plan.Prune(current, desired)
	sort.Strings(got)
	want := []string{"/home/taro/.config/old"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestPrune_EmptyWhenEverythingDesired(t *testing.T) {
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.gitconfig": {Tool: "git"},
	}}
	desired := map[string]bool{"/home/taro/.gitconfig": true}

	got := plan.Prune(current, desired)
	if len(got) != 0 {
		t.Fatalf("expected no prunes, got %v", got)
	}
}
