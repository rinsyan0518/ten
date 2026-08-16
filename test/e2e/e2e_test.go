package e2e_test

import (
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/dockertest"
)

func TestApply_MultiToolDAGOrder(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles", "work")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }

[tools.git-work]
enabled    = false
depends_on = ["git"]
links = { "home:.gitconfig.local" = "git/.gitconfig.work" }
`)
	sb.WriteFile(t, home+"/dotfiles/ten.work.toml", `
[tools.git-work]
enabled = true
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig.work", "work\n")

	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("ten apply failed (exit %d): %s", code, out)
	}

	for _, path := range []string{home + "/.gitconfig", home + "/.gitconfig.local"} {
		if isLink, _, ok := sb.Lstat(t, path); !ok || !isLink {
			t.Fatalf("expected %s to be a symlink", path)
		}
	}
	if strings.Index(out, "  git\n") > strings.Index(out, "  git-work\n") {
		t.Fatalf("expected git to apply before git-work in output: %s", out)
	}
}

func TestApply_UnenabledToolIsNotApplied(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }

[tools.nvim]
enabled = false
links = { "xdg:nvim" = "nvim" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("ten apply failed (exit %d): %s", code, out)
	}
	if _, _, ok := sb.Lstat(t, home+"/.config/nvim"); ok {
		t.Fatalf("expected nvim to NOT be applied since it has enabled = false")
	}
}

func TestApply_GroupedPlanOutputFormat(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }

[tools.nvim]
links = { "xdg:nvim" = "nvim" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")
	sb.WriteFile(t, home+"/dotfiles/nvim/init.lua", "-- nvim config\n")

	out, code := sb.Run(t, home, "apply", "--dry-run")
	if code != 0 {
		t.Fatalf("ten apply --dry-run failed (exit %d): %s", code, out)
	}
	if !strings.HasPrefix(out, "Plan: 2 to create") {
		t.Fatalf("expected summary line, got: %s", out)
	}
	if !strings.Contains(out, "  git\n") || !strings.Contains(out, "  nvim\n") {
		t.Fatalf("expected grouped tool headers, got: %s", out)
	}
}

func TestApply_PrunesResourceRemovedFromConfig(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }

[tools.nvim]
links = { "xdg:nvim" = "nvim" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")
	sb.WriteFile(t, home+"/dotfiles/nvim/init.lua", "-- nvim\n")

	if _, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("first apply failed")
	}
	if isLink, _, ok := sb.Lstat(t, home+"/.config/nvim"); !ok || !isLink {
		t.Fatalf("expected nvim symlink to exist after first apply")
	}

	// Remove nvim from desired config, then re-apply.
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("second apply failed (exit %d): %s", code, out)
	}
	if _, _, ok := sb.Lstat(t, home+"/.config/nvim"); ok {
		t.Fatalf("expected nvim symlink to be pruned after removing it from config")
	}
	if !strings.Contains(out, "- remove symlink") {
		t.Fatalf("expected prune output to name the removed resource type, got: %s", out)
	}
}

func TestApply_PruneRestoresBackupInsteadOfDeleting(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }

[tools.old]
links = { "home:.oldrc" = "old/.oldrc" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")
	sb.WriteFile(t, home+"/dotfiles/old/.oldrc", "managed oldrc\n")
	// A real user file that apply backs up when it takes the target over.
	sb.WriteFile(t, home+"/.oldrc", "original oldrc\n")

	if out, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("first apply failed (exit %d): %s", code, out)
	}

	// Drop the old tool from the config so its target gets pruned.
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("second apply failed (exit %d): %s", code, out)
	}

	if isLink, _, ok := sb.Lstat(t, home+"/.oldrc"); !ok || isLink {
		t.Fatalf("expected the pruned target to be the restored original file (isLink=%v ok=%v): %s", isLink, ok, out)
	}
	if got := sb.ReadFile(t, home+"/.oldrc"); got != "original oldrc\n" {
		t.Fatalf("expected prune to restore the backed-up original file, got %q", got)
	}
	if !strings.Contains(out, "+ restore backup") {
		t.Fatalf("expected prune output to show a backup restore, got: %s", out)
	}
	stateJSON := sb.ReadFile(t, home+"/.local/state/ten/ten.state.json")
	if strings.Contains(stateJSON, home+"/.oldrc") {
		t.Fatalf("expected the pruned target to be dropped from state, got: %s", stateJSON)
	}
}

func TestApply_ConvertingLinkToTemplateDoesNotWriteThroughSymlink(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.local.toml", `
[vars]
git_email = "taro@example.com"
`)
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig.local" = "git/gitconfig.local" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/gitconfig.local", "checked-in source\n")

	if out, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("first apply failed (exit %d): %s", code, out)
	}
	if isLink, _, ok := sb.Lstat(t, home+"/.gitconfig.local"); !ok || !isLink {
		t.Fatalf("expected .gitconfig.local to be a symlink after the first apply")
	}

	// Same tool, same target, now managed as a template instead of a link.
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
templates = { "home:.gitconfig.local" = "git/gitconfig.local.tmpl" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/gitconfig.local.tmpl", "email = {{ .Vars.git_email }}\n")

	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("second apply failed (exit %d): %s", code, out)
	}

	if got := sb.ReadFile(t, home+"/dotfiles/git/gitconfig.local"); got != "checked-in source\n" {
		t.Fatalf("the dotfiles repo source was written through ten's own symlink, got %q", got)
	}
	if isLink, _, ok := sb.Lstat(t, home+"/.gitconfig.local"); !ok || isLink {
		t.Fatalf("expected .gitconfig.local to be a real rendered file now (isLink=%v ok=%v)", isLink, ok)
	}
	if got := sb.ReadFile(t, home+"/.gitconfig.local"); got != "email = taro@example.com\n" {
		t.Fatalf("unexpected rendered content: %q", got)
	}

	findOut, _, _ := sb.Exec(t, "find "+home+"/.ten_backup -name .gitconfig.local")
	backup := strings.TrimSpace(findOut)
	if backup == "" {
		t.Fatalf("expected the replaced symlink to be backed up under .ten_backup, find output: %q", findOut)
	}
	if _, _, code := sb.Exec(t, "test -L "+backup); code != 0 {
		t.Fatalf("expected the backup %s to be the old symlink", backup)
	}
}

func TestApply_PruneSkipsTargetUserReplacedWithTheirOwnFile(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }

[tools.old]
links = { "home:.oldrc" = "old/.oldrc" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")
	sb.WriteFile(t, home+"/dotfiles/old/.oldrc", "managed oldrc\n")

	if out, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("first apply failed (exit %d): %s", code, out)
	}

	// The user replaced ten's symlink with a real file of their own.
	sb.Exec(t, "rm "+home+"/.oldrc")
	sb.WriteFile(t, home+"/.oldrc", "hand written by the user\n")

	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("second apply failed (exit %d): %s", code, out)
	}

	if got := sb.ReadFile(t, home+"/.oldrc"); got != "hand written by the user\n" {
		t.Fatalf("prune destroyed a file ten no longer owns, got %q", got)
	}
	if !strings.Contains(out, "warning") || !strings.Contains(out, home+"/.oldrc") {
		t.Fatalf("expected a warning naming the skipped target, got: %s", out)
	}
	stateJSON := sb.ReadFile(t, home+"/.local/state/ten/ten.state.json")
	if !strings.Contains(stateJSON, home+"/.oldrc") {
		t.Fatalf("expected the skipped target to stay tracked in state, got: %s", stateJSON)
	}
}

func TestDestroy_SkipsTargetUserReplacedWithTheirOwnFile(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	if out, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("apply failed (exit %d): %s", code, out)
	}

	sb.Exec(t, "rm "+home+"/.gitconfig")
	sb.WriteFile(t, home+"/.gitconfig", "hand written by the user\n")

	out, code := sb.Run(t, home, "destroy")
	if code != 0 {
		t.Fatalf("destroy failed (exit %d): %s", code, out)
	}
	if got := sb.ReadFile(t, home+"/.gitconfig"); got != "hand written by the user\n" {
		t.Fatalf("destroy deleted a file ten no longer owns, got %q", got)
	}
	if !strings.Contains(out, "warning") || !strings.Contains(out, home+"/.gitconfig") {
		t.Fatalf("expected a warning naming the skipped target, got: %s", out)
	}
	stateJSON := sb.ReadFile(t, home+"/.local/state/ten/ten.state.json")
	if !strings.Contains(stateJSON, home+"/.gitconfig") {
		t.Fatalf("expected the skipped target to stay tracked in state, got: %s", stateJSON)
	}
}

func TestApply_ErrorsWhenDotfilesRootIsUnset(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	out, code := sb.Run(t, home, "apply")
	if code == 0 {
		t.Fatalf("expected apply to fail when dotfiles_root is unset, got exit 0: %s", out)
	}
	if !strings.Contains(out, "dotfiles_root") {
		t.Fatalf("expected the error to name dotfiles_root, got: %s", out)
	}
}

func TestApply_ErrorsWhenRepoConfigMissingInsteadOfPruningEverything(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	if out, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("first apply failed (exit %d): %s", code, out)
	}

	// Simulate the misconfigured/not-yet-cloned repo case: the desired
	// state would resolve to nothing, which must not prune everything.
	sb.Exec(t, "rm "+home+"/dotfiles/ten.toml")

	out, code := sb.Run(t, home, "apply")
	if code == 0 {
		t.Fatalf("expected apply to fail when no repo config file is found, got exit 0: %s", out)
	}
	if isLink, _, ok := sb.Lstat(t, home+"/.gitconfig"); !ok || !isLink {
		t.Fatalf("expected the managed symlink to survive a misconfigured apply")
	}
	stateJSON := sb.ReadFile(t, home+"/.local/state/ten/ten.state.json")
	if !strings.Contains(stateJSON, home+"/.gitconfig") {
		t.Fatalf("expected state to still track the resource, got: %s", stateJSON)
	}
}

func TestApply_ErrorsWhenDotfilesRootDoesNotExist(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.Exec(t, "rm -rf "+home+"/dotfiles")

	out, code := sb.Run(t, home, "apply")
	if code == 0 {
		t.Fatalf("expected apply to fail when dotfiles_root does not exist, got exit 0: %s", out)
	}
	if !strings.Contains(out, home+"/dotfiles") {
		t.Fatalf("expected the error to name the missing directory, got: %s", out)
	}
}

func TestApply_ErrorsFriendlyWhenDotfilesRootIsAFile(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.Exec(t, "rm -rf "+home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles", "not a directory\n")

	out, code := sb.Run(t, home, "apply")
	if code == 0 {
		t.Fatalf("expected apply to fail when dotfiles_root is a file, got exit 0: %s", out)
	}
	if !strings.Contains(out, "is not an existing directory") {
		t.Fatalf("expected a friendly \"is not an existing directory\" error, got a raw error instead: %s", out)
	}
}

func TestApply_FailFastRetainsUntouchedResourcesInState(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git-work]
templates = { "home:.gitconfig" = "git/.gitconfig.tmpl" }

[tools.nvim]
links = { "xdg:nvim" = "nvim" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig.tmpl", "# git config\n")
	sb.WriteFile(t, home+"/dotfiles/nvim/init.lua", "-- nvim\n")

	if _, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("first apply failed")
	}
	if isLink, _, ok := sb.Lstat(t, home+"/.config/nvim"); !ok || !isLink {
		t.Fatalf("expected nvim symlink to exist after first apply")
	}

	// Remove git-work's template source so its apply fails on re-apply.
	// git-work sorts before nvim alphabetically, so the failure happens
	// before nvim's (already-correct) resource is ever reached again.
	sb.Exec(t, "rm "+home+"/dotfiles/git/.gitconfig.tmpl")

	out, code := sb.Run(t, home, "apply")
	if code == 0 {
		t.Fatalf("expected second apply to fail, got exit 0: %s", out)
	}
	if isLink, _, ok := sb.Lstat(t, home+"/.config/nvim"); !ok || !isLink {
		t.Fatalf("expected nvim symlink to still exist on disk after the failed apply")
	}
	stateJSON := sb.ReadFile(t, home+"/.local/state/ten/ten.state.json")
	if !strings.Contains(stateJSON, home+"/.config/nvim") {
		t.Fatalf("expected state to still track nvim's target after fail-fast, got: %s", stateJSON)
	}
}

func TestApply_HookFailureStopsLaterToolsFailFast(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	// aaa's pre_apply fails, so neither aaa's own link nor anything in the
	// dependent tool zzz may be applied.
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.aaa]
pre_apply = "exit 3"
links = { "home:.aaa" = "a/.a" }

[tools.zzz]
depends_on = ["aaa"]
links = { "home:.zzz" = "z/.z" }
post_apply = "touch `+home+`/zzz-marker"
`)
	sb.WriteFile(t, home+"/dotfiles/a/.a", "a\n")
	sb.WriteFile(t, home+"/dotfiles/z/.z", "z\n")

	out, code := sb.Run(t, home, "apply")
	if code == 0 {
		t.Fatalf("expected apply to fail when a pre_apply hook fails, got exit 0: %s", out)
	}
	if _, _, ok := sb.Lstat(t, home+"/.aaa"); ok {
		t.Fatalf("expected aaa's link NOT to be created after its pre_apply failed")
	}
	if _, _, ok := sb.Lstat(t, home+"/.zzz"); ok {
		t.Fatalf("expected the later tool zzz to never be applied (fail-fast)")
	}
	if _, _, ok := sb.Lstat(t, home+"/zzz-marker"); ok {
		t.Fatalf("expected the later tool zzz's post_apply hook to never run (fail-fast)")
	}
}

func TestApply_ReportsWhatWasAppliedBeforeAFailure(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	// aaa applies cleanly, then zzz fails on a missing template source.
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.aaa]
links = { "home:.aaa" = "a/.a" }

[tools.zzz]
depends_on = ["aaa"]
templates = { "home:.zzz" = "z/missing.tmpl" }
`)
	sb.WriteFile(t, home+"/dotfiles/a/.a", "a\n")

	out, code := sb.Run(t, home, "apply")
	if code == 0 {
		t.Fatalf("expected apply to fail on the missing template source: %s", out)
	}
	if isLink, _, ok := sb.Lstat(t, home+"/.aaa"); !ok || !isLink {
		t.Fatalf("expected aaa's symlink to have been created before the failure")
	}
	if !strings.Contains(out, "create symlink") || !strings.Contains(out, home+"/.aaa") {
		t.Fatalf("expected the failed run to still report the symlink it created, got: %s", out)
	}
}

func TestCLI_RejectsUnexpectedPositionalArguments(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	// Targeted apply/destroy does not exist, so an extra argument is a
	// typo and must not silently run against everything.
	out, code := sb.Run(t, home, "apply", "somearg")
	if code == 0 {
		t.Fatalf("expected `ten apply somearg` to fail, got exit 0: %s", out)
	}
	if !strings.Contains(out, "somearg") {
		t.Fatalf("expected the error to name the unexpected argument, got: %s", out)
	}
	if _, _, ok := sb.Lstat(t, home+"/.gitconfig"); ok {
		t.Fatalf("expected `ten apply somearg` not to apply anything")
	}

	if out, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("apply failed (exit %d): %s", code, out)
	}

	out, code = sb.Run(t, home, "destroy", "somearg")
	if code == 0 {
		t.Fatalf("expected `ten destroy somearg` to fail, got exit 0: %s", out)
	}
	if !strings.Contains(out, "somearg") {
		t.Fatalf("expected the error to name the unexpected argument, got: %s", out)
	}
	if isLink, _, ok := sb.Lstat(t, home+"/.gitconfig"); !ok || !isLink {
		t.Fatalf("expected `ten destroy somearg` not to destroy anything")
	}
}

func TestInit_SandboxInitWithEmptyProfileClearsProfile(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles", "work")
	stateJSON := sb.ReadFile(t, home+"/.local/state/ten/ten.state.json")
	if !strings.Contains(stateJSON, `"profile": "work"`) {
		t.Fatalf("expected profile to be set to work, got: %s", stateJSON)
	}

	// An empty profile arg must round-trip through Sandbox.Run's shell
	// quoting intact rather than being dropped, or --profile would be
	// left without a value and cobra would fail to parse it.
	sb.Init(t, home, home+"/dotfiles", "")
	stateJSON = sb.ReadFile(t, home+"/.local/state/ten/ten.state.json")
	if strings.Contains(stateJSON, `"profile"`) {
		t.Fatalf("expected profile to be cleared, got: %s", stateJSON)
	}
}

func TestDestroy_NoOpsWhenDotfilesRootIsGoneAndNothingIsManaged(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.Exec(t, "rm -rf "+home+"/dotfiles")

	out, code := sb.Run(t, home, "destroy")
	if code != 0 {
		t.Fatalf("expected destroy to no-op successfully when nothing is managed, even with dotfiles_root gone, got exit %d: %s", code, out)
	}
}

func TestDestroy_RemovesManagedSymlink(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles", "work")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	if _, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("apply failed")
	}
	out, code := sb.Run(t, home, "destroy")
	if code != 0 {
		t.Fatalf("destroy failed (exit %d): %s", code, out)
	}
	if _, _, ok := sb.Lstat(t, home+"/.gitconfig"); ok {
		t.Fatalf("expected .gitconfig to be removed by destroy")
	}
	// destroy's saved state must still carry the bootstrap fields forward,
	// or the next command would force the user to re-run `ten init`.
	stateJSON := sb.ReadFile(t, home+"/.local/state/ten/ten.state.json")
	if !strings.Contains(stateJSON, home+"/dotfiles") {
		t.Fatalf("expected dotfiles_root to survive destroy in state, got: %s", stateJSON)
	}
	if !strings.Contains(stateJSON, `"profile": "work"`) {
		t.Fatalf("expected profile to survive destroy in state, got: %s", stateJSON)
	}
}

func TestDestroy_RecoversManagedResourcesWhenDotfilesRootIsGone(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	if _, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("apply failed")
	}

	// Simulate the disaster-recovery scenario this feature exists for: the
	// dotfiles repository (and therefore ten.toml) is gone, but
	// ten.state.json under $XDG_STATE_HOME still knows what's managed.
	sb.Exec(t, "rm -rf "+home+"/dotfiles")

	out, code := sb.Run(t, home, "destroy")
	if code != 0 {
		t.Fatalf("expected destroy to succeed without a readable config, got exit %d: %s", code, out)
	}
	if _, _, ok := sb.Lstat(t, home+"/.gitconfig"); ok {
		t.Fatalf("expected .gitconfig to be removed by destroy even though dotfiles_root is gone")
	}
}

func TestDestroy_RestoresBackup(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")
	sb.WriteFile(t, home+"/.gitconfig", "original user config\n")

	if _, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("apply failed")
	}
	out, code := sb.Run(t, home, "destroy")
	if code != 0 {
		t.Fatalf("destroy failed (exit %d): %s", code, out)
	}
	got := sb.ReadFile(t, home+"/.gitconfig")
	if got != "original user config\n" {
		t.Fatalf("expected original file restored, got %q", got)
	}
}

func TestDestroy_RestoresDirectoryBackup(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.nvim]
links = { "xdg:nvim" = "nvim" }
`)
	sb.WriteFile(t, home+"/dotfiles/nvim/init.lua", "-- managed nvim\n")
	// A real pre-existing config *directory* that apply must back up and
	// destroy must put back.
	sb.WriteFile(t, home+"/.config/nvim/init.lua", "-- original nvim\n")
	sb.WriteFile(t, home+"/.config/nvim/lua/plugins.lua", "-- original plugins\n")

	if out, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("apply failed (exit %d): %s", code, out)
	}
	if isLink, _, ok := sb.Lstat(t, home+"/.config/nvim"); !ok || !isLink {
		t.Fatalf("expected .config/nvim to be a symlink after apply")
	}

	out, code := sb.Run(t, home, "destroy")
	if code != 0 {
		t.Fatalf("destroy failed (exit %d): %s", code, out)
	}
	if isLink, _, ok := sb.Lstat(t, home+"/.config/nvim"); !ok || isLink {
		t.Fatalf("expected .config/nvim to be a real directory again after destroy (isLink=%v ok=%v): %s", isLink, ok, out)
	}
	if got := sb.ReadFile(t, home+"/.config/nvim/init.lua"); got != "-- original nvim\n" {
		t.Fatalf("expected original nvim/init.lua restored, got %q", got)
	}
	if got := sb.ReadFile(t, home+"/.config/nvim/lua/plugins.lua"); got != "-- original plugins\n" {
		t.Fatalf("expected original nvim/lua/plugins.lua restored, got %q", got)
	}
}

func TestDestroy_RestoresBackupAfterSecondApply(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")
	sb.WriteFile(t, home+"/.gitconfig", "original user config\n")

	if _, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("first apply failed")
	}
	// A second, completely normal idempotent re-apply must not lose the
	// backup_path recorded by the first apply: Link returns Skipped with
	// no BackupPath since the symlink is already correct, so the state
	// write must fall back to the previously recorded backup_path rather
	// than clobbering it with an empty one.
	if out, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("second apply failed (exit %d): %s", code, out)
	}

	out, code := sb.Run(t, home, "destroy")
	if code != 0 {
		t.Fatalf("destroy failed (exit %d): %s", code, out)
	}
	got := sb.ReadFile(t, home+"/.gitconfig")
	if got != "original user config\n" {
		t.Fatalf("expected original file restored after a second apply, got %q", got)
	}
}

func TestDestroy_FailFastRetainsUntouchedResourcesInState(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.a]
links = { "home:.a" = "a/.a" }

[tools.b]
depends_on = ["a"]
links = { "home:.b" = "b/.b" }

[tools.c]
depends_on = ["b"]
links = { "home:.c" = "c/.c" }
`)
	sb.WriteFile(t, home+"/dotfiles/a/.a", "a\n")
	sb.WriteFile(t, home+"/dotfiles/b/.b", "b\n")
	sb.WriteFile(t, home+"/dotfiles/c/.c", "c\n")

	if _, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("apply failed")
	}

	// Sabotage b's tracked resource with a backup_path that doesn't exist
	// on disk, so destroy's restore-from-backup step for b fails partway
	// through the run. Destroy order is deterministic alphabetical order
	// by tool name (a, b, c): a is destroyed successfully first, b's
	// restore then fails, and c is never reached.
	stateJSON := sb.ReadFile(t, home+"/.local/state/ten/ten.state.json")
	sabotaged := strings.Replace(stateJSON,
		`"source": "`+home+`/dotfiles/b/.b"`,
		`"source": "`+home+`/dotfiles/b/.b",
      "backup_path": "`+home+`/.ten_backup/nonexistent/bogus"`,
		1)
	if sabotaged == stateJSON {
		t.Fatalf("failed to sabotage state.json: source line for b not found in %s", stateJSON)
	}
	// WriteFile's heredoc requires a trailing newline to terminate cleanly.
	sb.WriteFile(t, home+"/.local/state/ten/ten.state.json", sabotaged+"\n")

	out, code := sb.Run(t, home, "destroy")
	if code == 0 {
		t.Fatalf("expected destroy to fail, got exit 0: %s", out)
	}

	if _, _, ok := sb.Lstat(t, home+"/.a"); ok {
		t.Fatalf("expected .a to be removed before the failure")
	}
	if isLink, _, ok := sb.Lstat(t, home+"/.b"); !ok || !isLink {
		t.Fatalf("expected .b to remain a symlink after its failed restore")
	}
	if isLink, _, ok := sb.Lstat(t, home+"/.c"); !ok || !isLink {
		t.Fatalf("expected .c to remain untouched (not yet reached)")
	}

	finalState := sb.ReadFile(t, home+"/.local/state/ten/ten.state.json")
	if strings.Contains(finalState, home+"/.a\"") {
		t.Fatalf("expected .a to be dropped from state after successful removal, got: %s", finalState)
	}
	if !strings.Contains(finalState, home+"/.b\"") {
		t.Fatalf("expected .b to still be tracked in state after its failed removal, got: %s", finalState)
	}
	if !strings.Contains(finalState, home+"/.c\"") {
		t.Fatalf("expected .c to still be tracked in state (never reached), got: %s", finalState)
	}
}

func TestDestroy_PlanOutputGroupsAndNamesTheBackupSource(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }

[tools.nvim]
links = { "xdg:nvim" = "nvim" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")
	sb.WriteFile(t, home+"/dotfiles/nvim/init.lua", "-- nvim\n")
	// Only .gitconfig pre-exists, so only it gets a backup to restore.
	sb.WriteFile(t, home+"/.gitconfig", "original user config\n")

	if out, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("apply failed (exit %d): %s", code, out)
	}

	out, code := sb.Run(t, home, "destroy", "--dry-run")
	if code != 0 {
		t.Fatalf("destroy --dry-run failed (exit %d): %s", code, out)
	}
	if !strings.HasPrefix(out, "Plan: 2 to destroy") {
		t.Fatalf("expected a `Plan: 2 to destroy` summary line, got: %s", out)
	}
	if !strings.Contains(out, "  git\n") || !strings.Contains(out, "  nvim\n") {
		t.Fatalf("expected output grouped by tool, got: %s", out)
	}
	if !strings.Contains(out, "- remove symlink") {
		t.Fatalf("expected the removal line to name the resource type, got: %s", out)
	}
	// The backup source is the user's only pre-flight confirmation of what
	// an irreversible restore is about to move back.
	if !strings.Contains(out, home+"/.gitconfig <- "+home+"/.ten_backup/") {
		t.Fatalf("expected the restore line to name the backup it comes from, got: %s", out)
	}
}

func TestDestroy_DryRunMakesNoChanges(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	if _, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("apply failed")
	}
	out, code := sb.Run(t, home, "destroy", "--dry-run")
	if code != 0 {
		t.Fatalf("destroy --dry-run failed (exit %d): %s", code, out)
	}
	if !strings.Contains(out, "dry-run") {
		t.Fatalf("expected dry-run output to say so, got: %s", out)
	}
	if isLink, _, ok := sb.Lstat(t, home+"/.gitconfig"); !ok || !isLink {
		t.Fatalf("expected .gitconfig to remain a symlink under --dry-run")
	}
}
