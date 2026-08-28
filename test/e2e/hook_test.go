package e2e_test

import (
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/testutil/tencli"
)

func TestApply_RunsAfterHook(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
after = "echo HOOK_STDOUT && touch `+home+`/after-marker"
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("ten apply failed (exit %d): %s", code, out)
	}
	if _, _, ok := sb.Lstat(t, home+"/after-marker"); !ok {
		t.Fatalf("expected after hook to have created the marker file, output: %s", out)
	}
	if !strings.Contains(out, "HOOK_STDOUT") {
		t.Fatalf("expected hook stdout to be streamed to the user, got: %s", out)
	}
	if !strings.Contains(out, "run after") {
		t.Fatalf("expected plan output to mention after, got: %s", out)
	}
}

func TestApply_RunsBeforeHookBeforeLinks(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	// before writes the source file the link then points at; if the hook
	// ran after the link (or not at all) the link source would be missing.
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
before = "mkdir -p `+home+`/dotfiles/git && echo generated > `+home+`/dotfiles/git/.gitconfig"
links = { "home:.gitconfig" = "git/.gitconfig" }
`)

	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("ten apply failed (exit %d): %s", code, out)
	}
	if got := sb.ReadFile(t, home+"/.gitconfig"); got != "generated\n" {
		t.Fatalf("expected before to have generated the link source first, got %q", got)
	}
	if !strings.Contains(out, "run before") {
		t.Fatalf("expected plan output to mention before, got: %s", out)
	}
}

func TestApply_RunsOnceHookOnlyOnFirstManagedResourceCreation(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
once  = "touch `+home+`/once-marker"
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("first ten apply failed (exit %d): %s", code, out)
	}
	if _, _, ok := sb.Lstat(t, home+"/once-marker"); !ok {
		t.Fatalf("expected once hook to have created the marker file on first apply, output: %s", out)
	}
	if !strings.Contains(out, "run once") {
		t.Fatalf("expected plan output to mention once on first apply, got: %s", out)
	}

	sb.Exec(t, "rm "+home+"/once-marker")

	out, code = sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("second ten apply failed (exit %d): %s", code, out)
	}
	if _, _, ok := sb.Lstat(t, home+"/once-marker"); ok {
		t.Fatalf("expected once hook NOT to run again on the second apply")
	}
	if strings.Contains(out, "run once") {
		t.Fatalf("expected plan output NOT to mention once on the second apply, got: %s", out)
	}
}

func TestApply_DryRunDoesNotExecuteHooks(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
after = "touch `+home+`/after-marker"
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	out, code := sb.Run(t, home, "apply", "--dry-run")
	if code != 0 {
		t.Fatalf("ten apply --dry-run failed (exit %d): %s", code, out)
	}
	if _, _, ok := sb.Lstat(t, home+"/after-marker"); ok {
		t.Fatalf("expected --dry-run NOT to execute the after hook")
	}
	if !strings.Contains(out, "run after") {
		t.Fatalf("expected --dry-run plan to still mention the after hook, got: %s", out)
	}
}

func TestApply_DryRunDoesNotExecuteOnceHookButShowsInPlan(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
once  = "touch `+home+`/once-marker"
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	out, code := sb.Run(t, home, "apply", "--dry-run")
	if code != 0 {
		t.Fatalf("ten apply --dry-run failed (exit %d): %s", code, out)
	}
	if _, _, ok := sb.Lstat(t, home+"/once-marker"); ok {
		t.Fatalf("expected --dry-run NOT to execute the once hook")
	}
	if !strings.Contains(out, "run once") {
		t.Fatalf("expected --dry-run plan to still mention the once hook, got: %s", out)
	}
}

func TestApply_HookFailureStopsRunFailFast(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
before = "exit 1"
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	out, code := sb.Run(t, home, "apply")
	if code == 0 {
		t.Fatalf("expected apply to fail when before fails, got exit 0: %s", out)
	}
	if _, _, ok := sb.Lstat(t, home+"/.gitconfig"); ok {
		t.Fatalf("expected no symlink to be created when the tool's before hook failed")
	}
}

func TestApply_HookFailureStopsLaterToolsFailFast(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	// aaa's before hook fails, so neither aaa's own link nor anything in the
	// dependent tool zzz may be applied.
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.aaa]
before = "exit 3"
links = { "home:.aaa" = "a/.a" }

[tools.zzz]
depends_on = ["aaa"]
links = { "home:.zzz" = "z/.z" }
after = "touch `+home+`/zzz-marker"
`)
	sb.WriteFile(t, home+"/dotfiles/a/.a", "a\n")
	sb.WriteFile(t, home+"/dotfiles/z/.z", "z\n")

	out, code := sb.Run(t, home, "apply")
	if code == 0 {
		t.Fatalf("expected apply to fail when a before hook fails, got exit 0: %s", out)
	}
	if _, _, ok := sb.Lstat(t, home+"/.aaa"); ok {
		t.Fatalf("expected aaa's link NOT to be created after its before hook failed")
	}
	if _, _, ok := sb.Lstat(t, home+"/.zzz"); ok {
		t.Fatalf("expected the later tool zzz to never be applied (fail-fast)")
	}
	if _, _, ok := sb.Lstat(t, home+"/zzz-marker"); ok {
		t.Fatalf("expected the later tool zzz's after hook to never run (fail-fast)")
	}
}

func TestApply_HooksRunInTheDotfilesRoot(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	// The hook's behavior must not depend on where ten was invoked; its
	// cwd is pinned to the dotfiles root, so a relative path in a hook
	// always means "relative to the repo".
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.probe]
before = "pwd > `+home+`/hook-cwd"
`)

	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("ten apply failed (exit %d): %s", code, out)
	}
	if got := strings.TrimSpace(sb.ReadFile(t, home+"/hook-cwd")); got != home+"/dotfiles" {
		t.Fatalf("hook ran with cwd %q, want %q", got, home+"/dotfiles")
	}
}
