package main

import (
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/apply"
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

	out := formatApplyPlan(result, true)

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

func TestFormatApplyPlan_CountsOnlyNonSkippedLinksAsToCreate(t *testing.T) {
	result := apply.Result{
		Outcomes: []apply.ToolOutcome{
			{
				Tool: "git",
				Links: []apply.LinkResult{
					{Target: "/home/taro/.gitconfig", Source: "/dotfiles/git/.gitconfig"},
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

	out := formatApplyPlan(result, false)

	if !strings.Contains(out, "Plan: 1 to create, 0 to prune") {
		t.Fatalf("expected exactly one resource to create, got:\n%s", out)
	}
	if !strings.Contains(out, "+ create symlink   /home/taro/.gitconfig -> /dotfiles/git/.gitconfig") {
		t.Fatalf("expected create line for git, got:\n%s", out)
	}
	if !strings.Contains(out, "= symlink (up to date)   /home/taro/.config/hoge -> /dotfiles/xdg_config/hoge") {
		t.Fatalf("expected up-to-date line for hoge, got:\n%s", out)
	}
}
