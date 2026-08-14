package apply_test

import (
	"errors"
	"io"
	"testing"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/config"
	"github.com/rinsyan0518/ten/internal/state"
)

func TestDestroy_RemovesInReverseDependencyOrder(t *testing.T) {
	merged := config.Merged{
		Tools: map[string]config.Tool{
			"git":      {},
			"git-work": {DependsOn: []string{"git"}},
		},
	}
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.gitconfig":       {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitconfig"},
		"/home/taro/.gitconfig.local": {Tool: "git-work", Type: "symlink", Source: "/dotfiles/git-work/.gitconfig.local"},
	}}

	fx := &fakeExecutor{}
	result, newState, err := apply.Destroy(apply.DestroyParams{
		Merged: merged, Current: current, Home: "/home/taro", Out: io.Discard, Executor: fx,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Outcomes) != 2 || result.Outcomes[0].Tool != "git-work" || result.Outcomes[1].Tool != "git" {
		t.Fatalf("expected git-work destroyed before git, got %+v", result.Outcomes)
	}
	if len(newState.ManagedResources) != 0 {
		t.Fatalf("expected all resources removed from state, got %+v", newState.ManagedResources)
	}
}

func TestDestroy_StopsOnUnlinkFailureAndKeepsPartialResult(t *testing.T) {
	merged := config.Merged{Tools: map[string]config.Tool{"git": {}, "nvim": {}}}
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.gitconfig":   {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitconfig"},
		"/home/taro/.config/nvim": {Tool: "nvim", Type: "symlink", Source: "/dotfiles/nvim"},
	}}
	unlinkErr := errors.New("boom")
	fx := &fakeExecutor{
		UnlinkFunc: func(req apply.UnlinkRequest, dryRun bool) (apply.UnlinkResult, error) {
			if req.Target == "/home/taro/.gitconfig" {
				return apply.UnlinkResult{}, unlinkErr
			}
			return apply.UnlinkResult{Target: req.Target}, nil
		},
	}

	// No depends_on between git and nvim, so DestroyOrder (reverse of the
	// alphabetical forward order) visits nvim before git.
	result, _, err := apply.Destroy(apply.DestroyParams{
		Merged: merged, Current: current, Home: "/home/taro", Out: io.Discard, Executor: fx,
	})
	if err == nil || !errors.Is(err, unlinkErr) {
		t.Fatalf("expected error wrapping %v, got %v", unlinkErr, err)
	}
	if len(result.Outcomes) != 1 || result.Outcomes[0].Tool != "nvim" {
		t.Fatalf("expected nvim's outcome kept before git failed, got %+v", result.Outcomes)
	}
}

func TestDestroy_PassesDryRunToExecutorAndSkipsStateWrites(t *testing.T) {
	merged := config.Merged{Tools: map[string]config.Tool{"git": {}}}
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.gitconfig": {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitconfig"},
	}}
	var gotDryRun bool
	fx := &fakeExecutor{
		UnlinkFunc: func(req apply.UnlinkRequest, dryRun bool) (apply.UnlinkResult, error) {
			gotDryRun = dryRun
			return apply.UnlinkResult{Target: req.Target}, nil
		},
	}
	_, newState, err := apply.Destroy(apply.DestroyParams{
		Merged: merged, Current: current, Home: "/home/taro", Out: io.Discard, DryRun: true, Executor: fx,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gotDryRun {
		t.Fatalf("expected dryRun=true to reach Executor.Unlink")
	}
	if _, ok := newState.ManagedResources["/home/taro/.gitconfig"]; !ok {
		t.Fatalf("dry-run must not remove from state")
	}
}
