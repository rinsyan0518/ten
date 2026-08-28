package apply_test

import (
	"errors"
	"io"
	"testing"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/plan"
	"github.com/rinsyan0518/ten/internal/state"
)

func TestExecuteDestroy_RemovesInPlanOrderAndClearsState(t *testing.T) {
	dp := plan.DestroyPlan{Tools: []plan.DestroyToolPlan{
		{Tool: "git", Steps: []plan.PruneStep{
			{Target: "/home/taro/.gitconfig", Type: "symlink", Action: plan.ActionRemove},
			{Target: "/home/taro/.gitignore", Type: "symlink", Action: plan.ActionRemove},
		}},
		{Tool: "zsh", Steps: []plan.PruneStep{
			{Target: "/home/taro/.zshrc", Type: "symlink", Action: plan.ActionRemove},
		}},
	}}
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.zshrc":     {Tool: "zsh", Type: "symlink", Source: "/dotfiles/zsh/.zshrc"},
		"/home/taro/.gitignore": {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitignore"},
		"/home/taro/.gitconfig": {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitconfig"},
	}}

	fx := &fakeExecutor{}
	result, newState, err := apply.ExecuteDestroy(apply.DestroyExecParams{
		Plan: dp, Current: current, Out: io.Discard, Executor: fx,
	})
	if err != nil {
		t.Fatalf("ExecuteDestroy: %v", err)
	}
	if len(result.Outcomes) != 2 || result.Outcomes[0].Tool != "git" || result.Outcomes[1].Tool != "zsh" {
		t.Fatalf("expected outcomes grouped per plan order, got %+v", result.Outcomes)
	}
	if len(result.Outcomes[0].Entries) != 2 {
		t.Fatalf("expected git's two targets destroyed, got %+v", result.Outcomes[0].Entries)
	}
	if len(newState.ManagedResources) != 0 {
		t.Fatalf("expected all resources removed from state, got %+v", newState.ManagedResources)
	}
}

func TestExecuteDestroy_StopsOnUnlinkFailureAndKeepsPartialResult(t *testing.T) {
	dp := plan.DestroyPlan{Tools: []plan.DestroyToolPlan{
		{Tool: "git", Steps: []plan.PruneStep{{Target: "/home/taro/.gitconfig", Type: "symlink", Action: plan.ActionRemove}}},
		{Tool: "nvim", Steps: []plan.PruneStep{{Target: "/home/taro/.config/nvim", Type: "symlink", Action: plan.ActionRemove}}},
	}}
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.gitconfig":   {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitconfig"},
		"/home/taro/.config/nvim": {Tool: "nvim", Type: "symlink", Source: "/dotfiles/nvim"},
	}}
	unlinkErr := errors.New("boom")
	fx := &fakeExecutor{
		UnlinkFunc: func(req apply.UnlinkRequest) (apply.UnlinkResult, error) {
			if req.Target == "/home/taro/.config/nvim" {
				return apply.UnlinkResult{}, unlinkErr
			}
			return apply.UnlinkResult{Target: req.Target}, nil
		},
	}

	result, newState, err := apply.ExecuteDestroy(apply.DestroyExecParams{
		Plan: dp, Current: current, Out: io.Discard, Executor: fx,
	})
	if err == nil || !errors.Is(err, unlinkErr) {
		t.Fatalf("expected error wrapping %v, got %v", unlinkErr, err)
	}
	if len(result.Outcomes) != 1 || result.Outcomes[0].Tool != "git" {
		t.Fatalf("expected git's outcome kept before nvim failed, got %+v", result.Outcomes)
	}
	if _, ok := newState.ManagedResources["/home/taro/.gitconfig"]; ok {
		t.Fatalf("git's removal must be reflected in the returned state")
	}
	if _, ok := newState.ManagedResources["/home/taro/.config/nvim"]; !ok {
		t.Fatalf("the failed target must stay in state")
	}
}

func TestExecuteDestroy_SkipStepIsWarnedAndKeptInState(t *testing.T) {
	dp := plan.DestroyPlan{Tools: []plan.DestroyToolPlan{
		{Tool: "git", Steps: []plan.PruneStep{{Target: "/home/taro/.gitconfig", Type: "symlink", Action: plan.ActionSkip, SkipReason: "no longer a symlink created by ten"}}},
	}}
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.gitconfig": {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitconfig"},
	}}

	var out testWriter
	fx := &fakeExecutor{}
	result, newState, err := apply.ExecuteDestroy(apply.DestroyExecParams{
		Plan: dp, Current: current, Out: &out, Executor: fx,
	})
	if err != nil {
		t.Fatalf("ExecuteDestroy: %v", err)
	}
	if len(fx.calls) != 0 {
		t.Fatalf("a planned skip must not reach the executor, got %v", fx.calls)
	}
	if len(result.Outcomes) != 0 {
		t.Fatalf("a tool with only skipped steps must produce no outcome, got %+v", result.Outcomes)
	}
	if !out.contains("no longer a symlink") {
		t.Fatalf("expected a warning naming the reason, got %q", out.String())
	}
	if _, ok := newState.ManagedResources["/home/taro/.gitconfig"]; !ok {
		t.Fatalf("skipped resource must stay in state")
	}
}

func TestBuildDestroy_OrdersToolsAndTargetsAlphabetically(t *testing.T) {
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.zshrc":     {Tool: "zsh", Type: "symlink", Source: "/dotfiles/zsh/.zshrc"},
		"/home/taro/.gitignore": {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitignore"},
		"/home/taro/.gitconfig": {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitconfig"},
	}}
	ins := &fakeInspectorFS{entries: map[string]plan.Entry{
		"/home/taro/.zshrc":     {Exists: true, IsSymlink: true, LinkDest: "/dotfiles/zsh/.zshrc"},
		"/home/taro/.gitignore": {Exists: true, IsSymlink: true, LinkDest: "/dotfiles/git/.gitignore"},
		"/home/taro/.gitconfig": {Exists: true, IsSymlink: true, LinkDest: "/dotfiles/git/.gitconfig"},
	}}

	dp, err := plan.BuildDestroy(current, ins)
	if err != nil {
		t.Fatalf("BuildDestroy: %v", err)
	}
	if len(dp.Tools) != 2 || dp.Tools[0].Tool != "git" || dp.Tools[1].Tool != "zsh" {
		t.Fatalf("expected alphabetical tool order (git, zsh), got %+v", dp.Tools)
	}
	git := dp.Tools[0].Steps
	if len(git) != 2 || git[0].Target != "/home/taro/.gitconfig" || git[1].Target != "/home/taro/.gitignore" {
		t.Fatalf("expected git's targets in alphabetical order, got %+v", git)
	}
	for _, s := range append(git, dp.Tools[1].Steps...) {
		if s.Action != plan.ActionRemove {
			t.Fatalf("expected remove actions for owned symlinks without backups, got %+v", s)
		}
	}
}

func TestBuildDestroy_PredictsSkipForDisownedSymlink(t *testing.T) {
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.gitconfig": {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitconfig"},
	}}
	// The user replaced ten's symlink with a real file.
	ins := &fakeInspectorFS{entries: map[string]plan.Entry{
		"/home/taro/.gitconfig": {Exists: true},
	}}

	dp, err := plan.BuildDestroy(current, ins)
	if err != nil {
		t.Fatalf("BuildDestroy: %v", err)
	}
	if len(dp.Tools) != 1 || len(dp.Tools[0].Steps) != 1 {
		t.Fatalf("expected one step, got %+v", dp.Tools)
	}
	step := dp.Tools[0].Steps[0]
	if step.Action != plan.ActionSkip || step.SkipReason == "" {
		t.Fatalf("expected a skip with reason for the disowned target, got %+v", step)
	}
}

// fakeInspectorFS mirrors the plan package's test fake for use here.
type fakeInspectorFS struct {
	entries map[string]plan.Entry
	files   map[string][]byte
}

func (f *fakeInspectorFS) Inspect(path string) (plan.Entry, error) {
	return f.entries[path], nil
}

func (f *fakeInspectorFS) ReadFile(path string) ([]byte, error) {
	content, ok := f.files[path]
	if !ok {
		return nil, errors.New("fakeInspectorFS: no file at " + path)
	}
	return content, nil
}
