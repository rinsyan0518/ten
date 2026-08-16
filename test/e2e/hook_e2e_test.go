package e2e_test

import (
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/dockertest"
)

func TestApply_RunsPostApplyHook(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
post_apply = "echo HOOK_STDOUT && touch `+home+`/post-apply-marker"
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("ten apply failed (exit %d): %s", code, out)
	}
	if _, _, ok := sb.Lstat(t, home+"/post-apply-marker"); !ok {
		t.Fatalf("expected post_apply hook to have created the marker file, output: %s", out)
	}
	if !strings.Contains(out, "HOOK_STDOUT") {
		t.Fatalf("expected hook stdout to be streamed to the user, got: %s", out)
	}
	if !strings.Contains(out, "run post_apply") {
		t.Fatalf("expected plan output to mention post_apply, got: %s", out)
	}
}

func TestApply_RunsPreApplyHookBeforeLinks(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	// pre_apply writes the source file the link then points at; if the hook
	// ran after the link (or not at all) the link source would be missing.
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
pre_apply = "mkdir -p `+home+`/dotfiles/git && echo generated > `+home+`/dotfiles/git/.gitconfig"
links = { "home:.gitconfig" = "git/.gitconfig" }
`)

	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("ten apply failed (exit %d): %s", code, out)
	}
	if got := sb.ReadFile(t, home+"/.gitconfig"); got != "generated\n" {
		t.Fatalf("expected pre_apply to have generated the link source first, got %q", got)
	}
	if !strings.Contains(out, "run pre_apply") {
		t.Fatalf("expected plan output to mention pre_apply, got: %s", out)
	}
}

func TestApply_DryRunDoesNotExecuteHooks(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
post_apply = "touch `+home+`/post-apply-marker"
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	out, code := sb.Run(t, home, "apply", "--dry-run")
	if code != 0 {
		t.Fatalf("ten apply --dry-run failed (exit %d): %s", code, out)
	}
	if _, _, ok := sb.Lstat(t, home+"/post-apply-marker"); ok {
		t.Fatalf("expected --dry-run NOT to execute the post_apply hook")
	}
	if !strings.Contains(out, "run post_apply") {
		t.Fatalf("expected --dry-run plan to still mention the post_apply hook, got: %s", out)
	}
}

func TestApply_HookFailureStopsRunFailFast(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
pre_apply = "exit 1"
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	out, code := sb.Run(t, home, "apply")
	if code == 0 {
		t.Fatalf("expected apply to fail when pre_apply fails, got exit 0: %s", out)
	}
	if _, _, ok := sb.Lstat(t, home+"/.gitconfig"); ok {
		t.Fatalf("expected no symlink to be created when the tool's pre_apply failed")
	}
}
