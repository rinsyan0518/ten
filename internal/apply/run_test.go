package apply_test

import (
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/config"
	"github.com/rinsyan0518/ten/internal/pathresolve"
	"github.com/rinsyan0518/ten/internal/state"
)

// fakeExecutor is a test double for apply.Executor: no method touches the
// real filesystem or spawns a real process. calls records the order in
// which each method fired, for tests that assert on execution order.
type fakeExecutor struct {
	calls              []string
	LinkFunc           func(target, source, backupDir string, dryRun bool) (apply.LinkResult, error)
	UnlinkFunc         func(req apply.UnlinkRequest, dryRun bool) (apply.UnlinkResult, error)
	RenderTemplateFunc func(target, source string, vars map[string]string, ten apply.SystemInfo, backupDir string, alreadyManaged, dryRun bool) (apply.TemplateResult, error)
	RunHookFunc        func(cmdStr string, out io.Writer, dryRun bool) error
}

func (f *fakeExecutor) Link(target, source, backupDir string, dryRun bool) (apply.LinkResult, error) {
	f.calls = append(f.calls, "link:"+target)
	if f.LinkFunc != nil {
		return f.LinkFunc(target, source, backupDir, dryRun)
	}
	return apply.LinkResult{Target: target, Source: source}, nil
}

func (f *fakeExecutor) Unlink(req apply.UnlinkRequest, dryRun bool) (apply.UnlinkResult, error) {
	f.calls = append(f.calls, "unlink:"+req.Target)
	if f.UnlinkFunc != nil {
		return f.UnlinkFunc(req, dryRun)
	}
	return apply.UnlinkResult{Target: req.Target}, nil
}

func (f *fakeExecutor) RenderTemplate(target, source string, vars map[string]string, ten apply.SystemInfo, backupDir string, alreadyManaged, dryRun bool) (apply.TemplateResult, error) {
	f.calls = append(f.calls, "template:"+target)
	if f.RenderTemplateFunc != nil {
		return f.RenderTemplateFunc(target, source, vars, ten, backupDir, alreadyManaged, dryRun)
	}
	return apply.TemplateResult{Target: target, Source: source}, nil
}

func (f *fakeExecutor) RunHook(cmdStr string, out io.Writer, dryRun bool) error {
	if cmdStr == "" {
		return nil
	}
	f.calls = append(f.calls, "hook:"+cmdStr)
	if f.RunHookFunc != nil {
		return f.RunHookFunc(cmdStr, out, dryRun)
	}
	return nil
}

func TestApply_RunsToolsInDependencyOrderWithHooksAndLinks(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git": {
				Links: map[string]string{"home:.gitconfig": "git/.gitconfig"},
				After: "echo git-done",
			},
			"git-work": {
				DependsOn: []string{"git"},
				Before:    "echo work-start",
				Templates: map[string]string{"home:.gitconfig.local": "git/gitconfig.local.tmpl"},
			},
		},
		Enabled: map[string]bool{"git": true, "git-work": true},
	}

	fx := &fakeExecutor{}
	result, newState, err := apply.Apply(apply.RunParams{
		Merged:   merged,
		Current:  state.State{ManagedResources: map[string]state.Resource{}},
		Env:      pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"},
		Out:      io.Discard,
		Executor: fx,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantCalls := []string{"link:/home/taro/.gitconfig", "hook:echo git-done", "hook:echo work-start", "template:/home/taro/.gitconfig.local"}
	if !reflect.DeepEqual(fx.calls, wantCalls) {
		t.Fatalf("got calls %v, want %v", fx.calls, wantCalls)
	}
	if len(result.Outcomes) != 2 || result.Outcomes[0].Tool != "git" || result.Outcomes[1].Tool != "git-work" {
		t.Fatalf("unexpected outcome order: %+v", result.Outcomes)
	}
	if got := newState.ManagedResources["/home/taro/.gitconfig"].Tool; got != "git" {
		t.Fatalf("expected .gitconfig tracked under git, got %q", got)
	}
}

func TestApply_RunsOnceHookWhenToolNewlyManagesASymlink(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git": {Links: map[string]string{"home:.gitconfig": "git/.gitconfig"}, Once: "echo first-time"},
		},
		Enabled: map[string]bool{"git": true},
	}

	fx := &fakeExecutor{}
	result, _, err := apply.Apply(apply.RunParams{
		Merged:   merged,
		Current:  state.State{ManagedResources: map[string]state.Resource{}},
		Env:      pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"},
		Out:      io.Discard,
		Executor: fx,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantCalls := []string{"link:/home/taro/.gitconfig", "hook:echo first-time"}
	if !reflect.DeepEqual(fx.calls, wantCalls) {
		t.Fatalf("got calls %v, want %v", fx.calls, wantCalls)
	}
	if len(result.Outcomes) != 1 || result.Outcomes[0].Once != "echo first-time" {
		t.Fatalf("expected once to be recorded in the outcome, got %+v", result.Outcomes)
	}
}

func TestApply_RunsOnceHookWhenToolNewlyManagesATemplate(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git": {Templates: map[string]string{"home:.gitconfig": "git/.gitconfig.tmpl"}, Once: "echo first-time"},
		},
		Enabled: map[string]bool{"git": true},
	}

	fx := &fakeExecutor{}
	result, _, err := apply.Apply(apply.RunParams{
		Merged:   merged,
		Current:  state.State{ManagedResources: map[string]state.Resource{}},
		Env:      pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"},
		Out:      io.Discard,
		Executor: fx,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Outcomes) != 1 || result.Outcomes[0].Once != "echo first-time" {
		t.Fatalf("expected once to fire for a newly-managed template, got %+v", result.Outcomes)
	}
}

func TestApply_SetsTenToolPerToolWhenRenderingTemplates(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git":  {Templates: map[string]string{"home:.gitconfig.local": "git/gitconfig.local.tmpl"}},
			"nvim": {Templates: map[string]string{"home:.config/nvim/local.lua": "nvim/local.lua.tmpl"}},
		},
		Enabled: map[string]bool{"git": true, "nvim": true},
	}

	gotTools := map[string]string{}
	fx := &fakeExecutor{
		RenderTemplateFunc: func(target, source string, vars map[string]string, ten apply.SystemInfo, backupDir string, alreadyManaged, dryRun bool) (apply.TemplateResult, error) {
			gotTools[target] = ten.Tool
			return apply.TemplateResult{Target: target, Source: source}, nil
		},
	}

	_, _, err := apply.Apply(apply.RunParams{
		Merged:   merged,
		Current:  state.State{ManagedResources: map[string]state.Resource{}},
		Env:      pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"},
		Ten:      apply.SystemInfo{Hostname: "test-host", Profile: "work"},
		Out:      io.Discard,
		Executor: fx,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gotTools["/home/taro/.gitconfig.local"]; got != "git" {
		t.Fatalf("git template got Ten.Tool = %q, want %q", got, "git")
	}
	if got := gotTools["/home/taro/.config/nvim/local.lua"]; got != "nvim" {
		t.Fatalf("nvim template got Ten.Tool = %q, want %q", got, "nvim")
	}
}

func TestApply_SkipsOnceHookWhenTargetAlreadyTrackedInState(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git": {Links: map[string]string{"home:.gitconfig": "git/.gitconfig"}, Once: "echo first-time"},
		},
		Enabled: map[string]bool{"git": true},
	}
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.gitconfig": {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitconfig"},
	}}

	fx := &fakeExecutor{
		// Already tracked and already correct on disk, exactly like a real
		// idempotent re-apply: Link reports Skipped=true.
		LinkFunc: func(target, source, backupDir string, dryRun bool) (apply.LinkResult, error) {
			return apply.LinkResult{Target: target, Source: source, Skipped: true}, nil
		},
	}
	result, _, err := apply.Apply(apply.RunParams{
		Merged: merged, Current: current, Env: pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"}, Out: io.Discard, Executor: fx,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range fx.calls {
		if c == "hook:echo first-time" {
			t.Fatalf("expected once NOT to fire for an already-tracked target, got calls %v", fx.calls)
		}
	}
	if len(result.Outcomes) != 1 {
		t.Fatalf("expected one outcome for the tool with a skipped link, got %+v", result.Outcomes)
	}
	if len(result.Outcomes[0].Links) != 1 || !result.Outcomes[0].Links[0].Skipped {
		t.Fatalf("expected the skipped link to be included in the outcome, got %+v", result.Outcomes[0].Links)
	}
}

func TestApply_SkipsOnceHookForAToolWithNoResources(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git": {Before: "echo start", Once: "echo first-time", After: "echo done"},
		},
		Enabled: map[string]bool{"git": true},
	}

	fx := &fakeExecutor{}
	result, _, err := apply.Apply(apply.RunParams{
		Merged:   merged,
		Current:  state.State{ManagedResources: map[string]state.Resource{}},
		Env:      pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"},
		Out:      io.Discard,
		Executor: fx,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantCalls := []string{"hook:echo start", "hook:echo done"}
	if !reflect.DeepEqual(fx.calls, wantCalls) {
		t.Fatalf("expected once NOT to fire for a hooks-only tool, got calls %v, want %v", fx.calls, wantCalls)
	}
	if len(result.Outcomes) != 1 || result.Outcomes[0].Once != "" {
		t.Fatalf("expected outcome.Once to stay empty, got %+v", result.Outcomes)
	}
}

func TestApply_PrunesResourcesNotInDesired(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git": {Links: map[string]string{"home:.gitconfig": "git/.gitconfig"}},
		},
		Enabled: map[string]bool{"git": true},
	}
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.gitconfig":       {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitconfig"},
		"/home/taro/.config/old-tool": {Tool: "old-tool", Type: "symlink", Source: "/dotfiles/old-tool/x"},
	}}

	fx := &fakeExecutor{}
	result, newState, err := apply.Apply(apply.RunParams{
		Merged: merged, Current: current, Env: pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"}, Out: io.Discard, Executor: fx,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Prunes) != 1 || result.Prunes[0].Result.Target != "/home/taro/.config/old-tool" {
		t.Fatalf("expected old-tool to be pruned, got %+v", result.Prunes)
	}
	if _, ok := newState.ManagedResources["/home/taro/.config/old-tool"]; ok {
		t.Fatalf("expected old-tool removed from state")
	}
}

func TestApply_StopsOnLinkFailureAndKeepsPartialResult(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git":  {Links: map[string]string{"home:.gitconfig": "git/.gitconfig"}},
			"nvim": {Links: map[string]string{"xdg:nvim": "nvim"}},
		},
		Enabled: map[string]bool{"git": true, "nvim": true},
	}
	linkErr := errors.New("boom")
	fx := &fakeExecutor{
		LinkFunc: func(target, source, backupDir string, dryRun bool) (apply.LinkResult, error) {
			if target == "/home/taro/.config/nvim" {
				return apply.LinkResult{}, linkErr
			}
			return apply.LinkResult{Target: target, Source: source}, nil
		},
	}

	result, _, err := apply.Apply(apply.RunParams{
		Merged: merged, Current: state.State{ManagedResources: map[string]state.Resource{}}, Env: pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"}, Out: io.Discard, Executor: fx,
	})
	if err == nil || !errors.Is(err, linkErr) {
		t.Fatalf("expected error wrapping %v, got %v", linkErr, err)
	}
	if len(result.Outcomes) != 1 || result.Outcomes[0].Tool != "git" {
		t.Fatalf("expected only git's outcome to be recorded, got %+v", result.Outcomes)
	}
}

func TestApply_PassesDryRunToExecutorAndSkipsStateWrites(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git": {Links: map[string]string{"home:.gitconfig": "git/.gitconfig"}},
		},
		Enabled: map[string]bool{"git": true},
	}
	var gotDryRun bool
	fx := &fakeExecutor{
		LinkFunc: func(target, source, backupDir string, dryRun bool) (apply.LinkResult, error) {
			gotDryRun = dryRun
			return apply.LinkResult{Target: target, Source: source}, nil
		},
	}
	_, newState, err := apply.Apply(apply.RunParams{
		Merged: merged, Current: state.State{ManagedResources: map[string]state.Resource{}}, Env: pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"}, Out: io.Discard, DryRun: true, Executor: fx,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gotDryRun {
		t.Fatalf("expected dryRun=true to reach Executor.Link")
	}
	if _, ok := newState.ManagedResources["/home/taro/.gitconfig"]; ok {
		t.Fatalf("dry-run must not write to state")
	}
}

func TestApply_IncludesBothChangedAndUpToDateToolsInOutcomes(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git":  {Links: map[string]string{"home:.gitconfig": "git/.gitconfig"}},
			"hoge": {Links: map[string]string{"home:.hogerc": "hoge/.hogerc"}},
		},
		Enabled: map[string]bool{"git": true, "hoge": true},
	}
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.hogerc": {Tool: "hoge", Type: "symlink", Source: "/dotfiles/hoge/.hogerc"},
	}}

	fx := &fakeExecutor{
		LinkFunc: func(target, source, backupDir string, dryRun bool) (apply.LinkResult, error) {
			if target == "/home/taro/.hogerc" {
				// Already tracked and already correct on disk.
				return apply.LinkResult{Target: target, Source: source, Skipped: true}, nil
			}
			return apply.LinkResult{Target: target, Source: source}, nil
		},
	}
	result, _, err := apply.Apply(apply.RunParams{
		Merged: merged, Current: current, Env: pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"}, Out: io.Discard, Executor: fx,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Outcomes) != 2 {
		t.Fatalf("expected both tools to appear in outcomes, got %+v", result.Outcomes)
	}

	byTool := map[string]apply.ToolOutcome{}
	for _, o := range result.Outcomes {
		byTool[o.Tool] = o
	}
	if len(byTool["git"].Links) != 1 || byTool["git"].Links[0].Skipped {
		t.Fatalf("expected git's link to be a non-skipped create, got %+v", byTool["git"].Links)
	}
	if len(byTool["hoge"].Links) != 1 || !byTool["hoge"].Links[0].Skipped {
		t.Fatalf("expected hoge's link to be skipped (already up to date), got %+v", byTool["hoge"].Links)
	}
}
