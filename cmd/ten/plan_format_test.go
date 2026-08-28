package main

import (
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/plan"
)

func TestFormatApplyPlan_ShowsUpToDateForSkippedLink(t *testing.T) {
	result := apply.Result{
		Outcomes: []apply.ToolOutcome{
			{
				Tool: "hoge",
				Links: []apply.LinkResult{
					{Target: "/home/taro/.config/hoge", Source: "/dotfiles/xdg_config/hoge", Skipped: true},
				},
			},
		},
	}

	out := formatApplyPlan(result)

	if !strings.Contains(out, "= symlink (up to date)   /home/taro/.config/hoge -> /dotfiles/xdg_config/hoge") {
		t.Fatalf("expected up-to-date line in output, got:\n%s", out)
	}
	if strings.Contains(out, "+ create symlink") {
		t.Fatalf("expected no create-symlink line for a skipped link, got:\n%s", out)
	}
	if !strings.Contains(out, "Plan: 0 to create, 0 to prune") {
		t.Fatalf("expected skipped link to be excluded from the create count, got:\n%s", out)
	}
}

func TestFormatApplyPlan_CountsOnlyNonSkippedResourcesAsToCreate(t *testing.T) {
	result := apply.Result{
		Outcomes: []apply.ToolOutcome{
			{
				Tool: "git",
				Links: []apply.LinkResult{
					{Target: "/home/taro/.gitconfig", Source: "/dotfiles/git/.gitconfig"},
				},
				Templates: []apply.TemplateResult{
					{Target: "/home/taro/.gitconfig.local", Source: "/dotfiles/git/tmpl", Skipped: true},
				},
			},
			{
				Tool: "hoge",
				Links: []apply.LinkResult{
					{Target: "/home/taro/.config/hoge", Source: "/dotfiles/xdg_config/hoge", Skipped: true},
				},
			},
		},
	}

	out := formatApplyPlan(result)

	if !strings.Contains(out, "Plan: 1 to create, 0 to prune") {
		t.Fatalf("expected exactly one resource to create, got:\n%s", out)
	}
	if !strings.Contains(out, "+ create symlink   /home/taro/.gitconfig -> /dotfiles/git/.gitconfig") {
		t.Fatalf("expected create line for git, got:\n%s", out)
	}
	if !strings.Contains(out, "= template (up to date)   /home/taro/.gitconfig.local <- /dotfiles/git/tmpl") {
		t.Fatalf("expected up-to-date line for the unchanged template, got:\n%s", out)
	}
	if !strings.Contains(out, "= symlink (up to date)   /home/taro/.config/hoge -> /dotfiles/xdg_config/hoge") {
		t.Fatalf("expected up-to-date line for hoge, got:\n%s", out)
	}
}

func TestFormatPlan_RendersDryRunFromThePlanItself(t *testing.T) {
	pl := plan.Plan{
		Prunes: []plan.PruneStep{
			{Target: "/home/taro/.config/old", Type: "symlink", Action: plan.ActionRemove},
			{Target: "/home/taro/.vimrc", Type: "symlink", BackupPath: "/home/taro/.ten_backup/x/.vimrc", Action: plan.ActionRestore},
			{Target: "/home/taro/.zshrc", Type: "symlink", Action: plan.ActionSkip, SkipReason: "no longer a symlink created by ten"},
		},
		Tools: []plan.ToolPlan{{
			Tool:   "git",
			Before: "echo before",
			Links: []plan.LinkStep{
				{Target: "/home/taro/.gitconfig", Source: "/dotfiles/git/.gitconfig", Action: plan.ActionCreate},
				{Target: "/home/taro/.gitignore", Source: "/dotfiles/git/.gitignore", Action: plan.ActionNoop},
				{Target: "/home/taro/.gitattributes", Source: "/dotfiles/git/.gitattributes", Action: plan.ActionReplace},
			},
			Templates: []plan.TemplateStep{
				{Target: "/home/taro/.gitconfig.local", Source: "/dotfiles/git/tmpl", Action: plan.ActionUpdate},
			},
			Once:  "echo once",
			After: "echo after",
		}},
	}

	out := formatPlan(pl)

	// Counts: create+replace links (2) + update template (1); skip excluded from prunes (2).
	if !strings.Contains(out, "Plan: 3 to create, 2 to prune") {
		t.Fatalf("expected counts derived from actions, got:\n%s", out)
	}
	for _, want := range []string{
		"> run before (dry-run)",
		"+ create symlink (dry-run)   /home/taro/.gitconfig -> /dotfiles/git/.gitconfig",
		"= symlink (up to date)   /home/taro/.gitignore -> /dotfiles/git/.gitignore",
		"~ replace symlink (dry-run)   /home/taro/.gitattributes -> /dotfiles/git/.gitattributes",
		"~ render template (dry-run)  /home/taro/.gitconfig.local <- /dotfiles/git/tmpl",
		"> run once (dry-run)",
		"> run after (dry-run)",
		"- remove symlink (dry-run)    /home/taro/.config/old",
		"+ restore backup (dry-run)    /home/taro/.vimrc <- /home/taro/.ten_backup/x/.vimrc",
		"! skip    /home/taro/.zshrc (no longer a symlink created by ten)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in dry-run plan, got:\n%s", want, out)
		}
	}
}

func TestFormatPlan_EmptyPlan(t *testing.T) {
	if out := formatPlan(plan.Plan{}); out != "Plan: 0 to create\n" {
		t.Fatalf("got %q", out)
	}
}

func TestFormatDestroyDryRun_RendersFromThePlanItself(t *testing.T) {
	dp := plan.DestroyPlan{Tools: []plan.DestroyToolPlan{{
		Tool: "git",
		Steps: []plan.PruneStep{
			{Target: "/home/taro/.gitconfig", Type: "symlink", Action: plan.ActionRemove},
			{Target: "/home/taro/.gitconfig.local", Type: "template", BackupPath: "/b/x", Action: plan.ActionRestore},
			{Target: "/home/taro/.gitignore", Type: "symlink", Action: plan.ActionSkip, SkipReason: "symlink now points at /elsewhere, not /dotfiles/git/.gitignore"},
		},
	}}}

	out := formatDestroyDryRun(dp)

	if !strings.Contains(out, "Plan: 2 to destroy") {
		t.Fatalf("expected skip excluded from the destroy count, got:\n%s", out)
	}
	for _, want := range []string{
		"- remove symlink (dry-run)    /home/taro/.gitconfig",
		"+ restore backup (dry-run)    /home/taro/.gitconfig.local <- /b/x",
		"! skip    /home/taro/.gitignore (symlink now points at /elsewhere, not /dotfiles/git/.gitignore)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in destroy dry-run, got:\n%s", want, out)
		}
	}
}
