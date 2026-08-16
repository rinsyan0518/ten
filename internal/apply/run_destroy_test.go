package apply_test

import (
	"errors"
	"io"
	"testing"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/state"
)

func TestDestroy_RemovesInDeterministicToolAndTargetOrder(t *testing.T) {
	// No depends_on relationship exists between these tools at all — destroy
	// no longer reads config, so there is nothing to express one with. Order
	// is purely alphabetical: tool name, then target name within a tool.
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.zshrc":     {Tool: "zsh", Type: "symlink", Source: "/dotfiles/zsh/.zshrc"},
		"/home/taro/.gitignore": {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitignore"},
		"/home/taro/.gitconfig": {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitconfig"},
	}}

	fx := &fakeExecutor{}
	result, newState, err := apply.Destroy(apply.DestroyParams{
		Current: current, Home: "/home/taro", Out: io.Discard, Executor: fx,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Outcomes) != 2 || result.Outcomes[0].Tool != "git" || result.Outcomes[1].Tool != "zsh" {
		t.Fatalf("expected tools destroyed in alphabetical order (git, zsh), got %+v", result.Outcomes)
	}
	gitTargets := result.Outcomes[0].Entries
	if len(gitTargets) != 2 || gitTargets[0].Result.Target != "/home/taro/.gitconfig" || gitTargets[1].Result.Target != "/home/taro/.gitignore" {
		t.Fatalf("expected git's targets destroyed in alphabetical order (.gitconfig, .gitignore), got %+v", gitTargets)
	}
	if len(newState.ManagedResources) != 0 {
		t.Fatalf("expected all resources removed from state, got %+v", newState.ManagedResources)
	}
}

func TestDestroy_StopsOnUnlinkFailureAndKeepsPartialResult(t *testing.T) {
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.gitconfig":   {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitconfig"},
		"/home/taro/.config/nvim": {Tool: "nvim", Type: "symlink", Source: "/dotfiles/nvim"},
	}}
	unlinkErr := errors.New("boom")
	fx := &fakeExecutor{
		UnlinkFunc: func(req apply.UnlinkRequest, dryRun bool) (apply.UnlinkResult, error) {
			if req.Target == "/home/taro/.config/nvim" {
				return apply.UnlinkResult{}, unlinkErr
			}
			return apply.UnlinkResult{Target: req.Target}, nil
		},
	}

	// Deterministic order is alphabetical by tool name, so git (succeeds) is
	// processed before nvim (fails).
	result, _, err := apply.Destroy(apply.DestroyParams{
		Current: current, Home: "/home/taro", Out: io.Discard, Executor: fx,
	})
	if err == nil || !errors.Is(err, unlinkErr) {
		t.Fatalf("expected error wrapping %v, got %v", unlinkErr, err)
	}
	if len(result.Outcomes) != 1 || result.Outcomes[0].Tool != "git" {
		t.Fatalf("expected git's outcome kept before nvim failed, got %+v", result.Outcomes)
	}
}

func TestDestroy_PassesDryRunToExecutorAndSkipsStateWrites(t *testing.T) {
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
		Current: current, Home: "/home/taro", Out: io.Discard, DryRun: true, Executor: fx,
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
