package apply_test

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/plan"
	"github.com/rinsyan0518/ten/internal/state"
)

// fakeExecutor is a test double for apply.Executor: no method touches the
// real filesystem or spawns a real process. calls records the order in
// which each method fired, for tests that assert on execution order.
type fakeExecutor struct {
	calls              []string
	LinkFunc           func(target, source, backupDir string) (apply.LinkResult, error)
	UnlinkFunc         func(req apply.UnlinkRequest) (apply.UnlinkResult, error)
	RenderTemplateFunc func(target, source string, vars map[string]string, ten apply.SystemInfo, backupDir string, backup bool) (apply.TemplateResult, error)
	RunHookFunc        func(cmdStr, dir string, out io.Writer) error
}

func (f *fakeExecutor) Link(target, source, backupDir string) (apply.LinkResult, error) {
	f.calls = append(f.calls, "link:"+target)
	if f.LinkFunc != nil {
		return f.LinkFunc(target, source, backupDir)
	}
	return apply.LinkResult{Target: target, Source: source}, nil
}

func (f *fakeExecutor) Unlink(req apply.UnlinkRequest) (apply.UnlinkResult, error) {
	f.calls = append(f.calls, "unlink:"+req.Target)
	if f.UnlinkFunc != nil {
		return f.UnlinkFunc(req)
	}
	return apply.UnlinkResult{Target: req.Target, Restored: req.BackupPath != ""}, nil
}

func (f *fakeExecutor) RenderTemplate(target, source string, vars map[string]string, ten apply.SystemInfo, backupDir string, backup bool) (apply.TemplateResult, error) {
	f.calls = append(f.calls, "template:"+target)
	if f.RenderTemplateFunc != nil {
		return f.RenderTemplateFunc(target, source, vars, ten, backupDir, backup)
	}
	return apply.TemplateResult{Target: target, Source: source}, nil
}

func (f *fakeExecutor) RunHook(cmdStr, dir string, out io.Writer) error {
	if cmdStr == "" {
		return nil
	}
	f.calls = append(f.calls, "hook:"+cmdStr)
	if f.RunHookFunc != nil {
		return f.RunHookFunc(cmdStr, dir, out)
	}
	return nil
}

func emptyState() state.State {
	return state.State{ManagedResources: map[string]state.Resource{}}
}

func TestExecute_WalksPlanInOrderAndRecordsState(t *testing.T) {
	pl := plan.Plan{
		Tools: []plan.ToolPlan{
			{
				Tool:  "git",
				Links: []plan.LinkStep{{Target: "/home/taro/.gitconfig", Source: "/dotfiles/git/.gitconfig", Action: plan.ActionCreate}},
				After: "echo git-done",
			},
			{
				Tool:      "git-work",
				Before:    "echo work-start",
				Templates: []plan.TemplateStep{{Target: "/home/taro/.gitconfig.local", Source: "/dotfiles/git/tmpl", Action: plan.ActionCreate}},
			},
		},
	}

	fx := &fakeExecutor{}
	result, newState, err := apply.Execute(apply.ExecParams{
		Plan: pl, Current: emptyState(), BackupDir: "/home/taro/.ten_backup", Out: io.Discard, Executor: fx,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	wantCalls := []string{"link:/home/taro/.gitconfig", "hook:echo git-done", "hook:echo work-start", "template:/home/taro/.gitconfig.local"}
	if !reflect.DeepEqual(fx.calls, wantCalls) {
		t.Fatalf("got calls %v, want %v", fx.calls, wantCalls)
	}
	if len(result.Outcomes) != 2 || result.Outcomes[0].Tool != "git" || result.Outcomes[1].Tool != "git-work" {
		t.Fatalf("unexpected outcome order: %+v", result.Outcomes)
	}
	if got := newState.ManagedResources["/home/taro/.gitconfig"]; got.Tool != "git" || got.Type != "symlink" {
		t.Fatalf("expected .gitconfig tracked under git, got %+v", got)
	}
	if got := newState.ManagedResources["/home/taro/.gitconfig.local"]; got.Tool != "git-work" || got.Type != "template" {
		t.Fatalf("expected .gitconfig.local tracked under git-work, got %+v", got)
	}
	if got := result.Outcomes[1].Templates[0].Source; got != "/dotfiles/git/tmpl" {
		t.Fatalf("template result should carry the step's source, got %q", got)
	}
}

func TestExecute_SkipsNoopLinkStepsWithoutTouchingExecutor(t *testing.T) {
	pl := plan.Plan{
		Tools: []plan.ToolPlan{{
			Tool:  "git",
			Links: []plan.LinkStep{{Target: "/home/taro/.gitconfig", Source: "/dotfiles/git/.gitconfig", Action: plan.ActionNoop}},
		}},
	}
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.gitconfig": {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitconfig"},
	}}

	fx := &fakeExecutor{}
	result, newState, err := apply.Execute(apply.ExecParams{
		Plan: pl, Current: current, BackupDir: "/b", Out: io.Discard, Executor: fx,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(fx.calls) != 0 {
		t.Fatalf("noop steps must not reach the executor, got calls %v", fx.calls)
	}
	if len(result.Outcomes) != 1 {
		t.Fatalf("expected the up-to-date tool to still appear in outcomes, got %+v", result.Outcomes)
	}
	o := result.Outcomes[0]
	if len(o.Links) != 1 || !o.Links[0].Skipped {
		t.Fatalf("expected a skipped link result, got %+v", o.Links)
	}
	if _, ok := newState.ManagedResources["/home/taro/.gitconfig"]; !ok {
		t.Fatalf("noop resources must stay tracked in state")
	}
}

func TestExecute_SetsTenToolPerToolWhenRenderingTemplates(t *testing.T) {
	pl := plan.Plan{
		Tools: []plan.ToolPlan{
			{Tool: "git", Templates: []plan.TemplateStep{{Target: "/home/taro/.a", Source: "/dotfiles/a.tmpl", Action: plan.ActionCreate}}},
			{Tool: "nvim", Templates: []plan.TemplateStep{{Target: "/home/taro/.b", Source: "/dotfiles/b.tmpl", Action: plan.ActionCreate}}},
		},
	}
	gotTools := map[string]string{}
	fx := &fakeExecutor{
		RenderTemplateFunc: func(target, source string, vars map[string]string, ten apply.SystemInfo, backupDir string, backup bool) (apply.TemplateResult, error) {
			gotTools[target] = ten.Tool
			return apply.TemplateResult{Target: target, Source: source}, nil
		},
	}

	_, _, err := apply.Execute(apply.ExecParams{
		Plan: pl, Current: emptyState(), BackupDir: "/b", Ten: apply.SystemInfo{Hostname: "h"}, Out: io.Discard, Executor: fx,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotTools["/home/taro/.a"] != "git" || gotTools["/home/taro/.b"] != "nvim" {
		t.Fatalf("Ten.Tool not set per tool: %+v", gotTools)
	}
}

func TestExecute_RecordsExecutorReportedUpToDateTemplate(t *testing.T) {
	pl := plan.Plan{
		Tools: []plan.ToolPlan{{
			Tool:      "git",
			Templates: []plan.TemplateStep{{Target: "/home/taro/.gitconfig.local", Source: "/dotfiles/git/tmpl", Action: plan.ActionUpdate}},
		}},
	}
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.gitconfig.local": {Tool: "git", Type: "template", Source: "/dotfiles/git/tmpl"},
	}}
	fx := &fakeExecutor{
		RenderTemplateFunc: func(target, source string, vars map[string]string, ten apply.SystemInfo, backupDir string, backup bool) (apply.TemplateResult, error) {
			return apply.TemplateResult{Target: target, Source: source, Skipped: true}, nil
		},
	}

	result, newState, err := apply.Execute(apply.ExecParams{
		Plan: pl, Current: current, BackupDir: "/b", Out: io.Discard, Executor: fx,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Outcomes) != 1 || len(result.Outcomes[0].Templates) != 1 || !result.Outcomes[0].Templates[0].Skipped {
		t.Fatalf("expected the skipped template in the outcome, got %+v", result.Outcomes)
	}
	if _, ok := newState.ManagedResources["/home/taro/.gitconfig.local"]; !ok {
		t.Fatalf("an up-to-date template must stay tracked in state")
	}
}

func TestExecute_PreservesRecordedBackupPathOnReapply(t *testing.T) {
	pl := plan.Plan{
		Tools: []plan.ToolPlan{{
			Tool:  "git",
			Links: []plan.LinkStep{{Target: "/home/taro/.gitconfig", Source: "/dotfiles/git/.gitconfig", Action: plan.ActionCreate}},
		}},
	}
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.gitconfig": {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitconfig", BackupPath: "/home/taro/.ten_backup/old/.gitconfig"},
	}}

	fx := &fakeExecutor{} // Link returns no BackupPath, like an idempotent re-link
	_, newState, err := apply.Execute(apply.ExecParams{
		Plan: pl, Current: current, BackupDir: "/b", Out: io.Discard, Executor: fx,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := newState.ManagedResources["/home/taro/.gitconfig"].BackupPath; got != "/home/taro/.ten_backup/old/.gitconfig" {
		t.Fatalf("recorded backup path must survive a re-apply, got %q", got)
	}
}

func TestExecute_RunsOnceHookWhenArmed(t *testing.T) {
	pl := plan.Plan{
		Tools: []plan.ToolPlan{{
			Tool:  "git",
			Links: []plan.LinkStep{{Target: "/home/taro/.gitconfig", Source: "/dotfiles/git/.gitconfig", Action: plan.ActionCreate}},
			Once:  "echo first-time",
		}},
	}

	fx := &fakeExecutor{}
	result, _, err := apply.Execute(apply.ExecParams{
		Plan: pl, Current: emptyState(), BackupDir: "/b", Out: io.Discard, Executor: fx,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	wantCalls := []string{"link:/home/taro/.gitconfig", "hook:echo first-time"}
	if !reflect.DeepEqual(fx.calls, wantCalls) {
		t.Fatalf("got calls %v, want %v", fx.calls, wantCalls)
	}
	if len(result.Outcomes) != 1 || result.Outcomes[0].Once != "echo first-time" {
		t.Fatalf("expected once recorded in the outcome, got %+v", result.Outcomes)
	}
}

func TestExecute_FailedOnceHookLeavesTargetsUntrackedSoOnceRefires(t *testing.T) {
	pl := plan.Plan{
		Tools: []plan.ToolPlan{{
			Tool: "git",
			Links: []plan.LinkStep{
				{Target: "/home/taro/.gitconfig", Source: "/dotfiles/git/.gitconfig", Action: plan.ActionCreate},
			},
			Once: "exit 1",
		}},
	}
	hookErr := errors.New("once failed")
	fx := &fakeExecutor{
		RunHookFunc: func(cmdStr, dir string, out io.Writer) error {
			if cmdStr == "exit 1" {
				return hookErr
			}
			return nil
		},
	}

	_, newState, err := apply.Execute(apply.ExecParams{
		Plan: pl, Current: emptyState(), BackupDir: "/b", Out: io.Discard, Executor: fx,
	})
	if err == nil || !errors.Is(err, hookErr) {
		t.Fatalf("expected the once failure to propagate, got %v", err)
	}
	// If the newly-managed target were recorded despite the once failure,
	// the next apply would see it as already tracked and never re-fire
	// once — the setup command would be silently lost forever.
	if _, ok := newState.ManagedResources["/home/taro/.gitconfig"]; ok {
		t.Fatalf("targets newly managed this run must stay untracked when their once hook fails")
	}
}

func TestExecute_FailedOnceHookKeepsPreviouslyTrackedTargets(t *testing.T) {
	// git already tracks .gitconfig from an earlier run; this run adds
	// .gitignore and once fails. Only the new target may be rolled back.
	pl := plan.Plan{
		Tools: []plan.ToolPlan{{
			Tool: "git",
			Links: []plan.LinkStep{
				{Target: "/home/taro/.gitconfig", Source: "/dotfiles/git/.gitconfig", Action: plan.ActionNoop},
				{Target: "/home/taro/.gitignore", Source: "/dotfiles/git/.gitignore", Action: plan.ActionCreate},
			},
			Once: "exit 1",
		}},
	}
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.gitconfig": {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitconfig"},
	}}
	hookErr := errors.New("once failed")
	fx := &fakeExecutor{
		RunHookFunc: func(cmdStr, dir string, out io.Writer) error {
			if cmdStr == "exit 1" {
				return hookErr
			}
			return nil
		},
	}

	_, newState, err := apply.Execute(apply.ExecParams{
		Plan: pl, Current: current, BackupDir: "/b", Out: io.Discard, Executor: fx,
	})
	if err == nil || !errors.Is(err, hookErr) {
		t.Fatalf("expected the once failure to propagate, got %v", err)
	}
	if _, ok := newState.ManagedResources["/home/taro/.gitconfig"]; !ok {
		t.Fatalf("previously tracked targets must survive a once failure")
	}
	if _, ok := newState.ManagedResources["/home/taro/.gitignore"]; ok {
		t.Fatalf("the newly managed target must be rolled back from tracking")
	}
}

func TestExecute_ExecutesPrunesBeforeToolsAndUpdatesState(t *testing.T) {
	pl := plan.Plan{
		Prunes: []plan.PruneStep{{Target: "/home/taro/.config/old", Type: "symlink", Action: plan.ActionRemove}},
		Tools: []plan.ToolPlan{{
			Tool:  "git",
			Links: []plan.LinkStep{{Target: "/home/taro/.gitconfig", Source: "/dotfiles/git/.gitconfig", Action: plan.ActionCreate}},
		}},
	}
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.config/old": {Tool: "old", Type: "symlink", Source: "/dotfiles/old/x"},
	}}

	fx := &fakeExecutor{}
	result, newState, err := apply.Execute(apply.ExecParams{
		Plan: pl, Current: current, BackupDir: "/b", Out: io.Discard, Executor: fx,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	wantCalls := []string{"unlink:/home/taro/.config/old", "link:/home/taro/.gitconfig"}
	if !reflect.DeepEqual(fx.calls, wantCalls) {
		t.Fatalf("got calls %v, want %v", fx.calls, wantCalls)
	}
	if len(result.Prunes) != 1 || result.Prunes[0].Result.Target != "/home/taro/.config/old" {
		t.Fatalf("expected the prune in the result, got %+v", result.Prunes)
	}
	if _, ok := newState.ManagedResources["/home/taro/.config/old"]; ok {
		t.Fatalf("pruned resource must leave state")
	}
}

func TestExecute_SkipStepIsWarnedAndKeptInState(t *testing.T) {
	pl := plan.Plan{
		Prunes: []plan.PruneStep{{Target: "/home/taro/.config/old", Type: "symlink", Action: plan.ActionSkip, SkipReason: "no longer a symlink created by ten"}},
	}
	current := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.config/old": {Tool: "old", Type: "symlink", Source: "/dotfiles/old/x"},
	}}

	var out testWriter
	fx := &fakeExecutor{}
	result, newState, err := apply.Execute(apply.ExecParams{
		Plan: pl, Current: current, BackupDir: "/b", Out: &out, Executor: fx,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(fx.calls) != 0 {
		t.Fatalf("a planned skip must not reach the executor, got %v", fx.calls)
	}
	if len(result.Prunes) != 0 {
		t.Fatalf("skipped prunes must not appear as prune outcomes, got %+v", result.Prunes)
	}
	if !out.contains("no longer a symlink") {
		t.Fatalf("expected a warning naming the reason, got %q", out.String())
	}
	if _, ok := newState.ManagedResources["/home/taro/.config/old"]; !ok {
		t.Fatalf("skipped resource must stay in state")
	}
}

func TestExecute_StopsOnLinkFailureAndKeepsPartialResult(t *testing.T) {
	pl := plan.Plan{
		Tools: []plan.ToolPlan{
			{Tool: "git", Links: []plan.LinkStep{{Target: "/home/taro/.gitconfig", Source: "/dotfiles/git/.gitconfig", Action: plan.ActionCreate}}},
			{Tool: "nvim", Links: []plan.LinkStep{{Target: "/home/taro/.config/nvim", Source: "/dotfiles/nvim", Action: plan.ActionCreate}}},
		},
	}
	linkErr := errors.New("boom")
	fx := &fakeExecutor{
		LinkFunc: func(target, source, backupDir string) (apply.LinkResult, error) {
			if target == "/home/taro/.config/nvim" {
				return apply.LinkResult{}, linkErr
			}
			return apply.LinkResult{Target: target, Source: source}, nil
		},
	}

	result, newState, err := apply.Execute(apply.ExecParams{
		Plan: pl, Current: emptyState(), BackupDir: "/b", Out: io.Discard, Executor: fx,
	})
	if err == nil || !errors.Is(err, linkErr) {
		t.Fatalf("expected error wrapping %v, got %v", linkErr, err)
	}
	if len(result.Outcomes) != 1 || result.Outcomes[0].Tool != "git" {
		t.Fatalf("expected only git's outcome kept, got %+v", result.Outcomes)
	}
	if _, ok := newState.ManagedResources["/home/taro/.gitconfig"]; !ok {
		t.Fatalf("work done before the failure must be recorded in state")
	}
}

// testWriter is a tiny in-memory io.Writer usable across tests.
type testWriter struct{ buf []byte }

func (w *testWriter) Write(p []byte) (int, error) { w.buf = append(w.buf, p...); return len(p), nil }
func (w *testWriter) String() string              { return string(w.buf) }
func (w *testWriter) contains(s string) bool      { return strings.Contains(w.String(), s) }
