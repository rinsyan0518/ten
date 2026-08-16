package e2e_test

import (
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/testutil/tencli"
)

func TestApply_CreatesSymlinkForSingleTool(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "[user]\n\tname = Taro\n")

	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("ten apply failed (exit %d): %s", code, out)
	}

	isLink, target, ok := sb.Lstat(t, home+"/.gitconfig")
	if !ok || !isLink {
		t.Fatalf("expected %s/.gitconfig to be a symlink, ok=%v isLink=%v", home, ok, isLink)
	}
	if target != home+"/dotfiles/git/.gitconfig" {
		t.Fatalf("unexpected symlink target: %q", target)
	}
	if !strings.Contains(out, "create symlink") {
		t.Fatalf("expected output to mention symlink creation, got: %s", out)
	}
}

func TestApply_BacksUpExistingFileBeforeLinking(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "[user]\n\tname = Taro\n")
	sb.WriteFile(t, home+"/.gitconfig", "old local config\n")

	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("ten apply failed (exit %d): %s", code, out)
	}

	isLink, target, ok := sb.Lstat(t, home+"/.gitconfig")
	if !ok || !isLink || target != home+"/dotfiles/git/.gitconfig" {
		t.Fatalf("expected .gitconfig to become a symlink, got isLink=%v target=%q ok=%v", isLink, target, ok)
	}

	findOut, _, code := sb.Exec(t, "find "+home+"/.ten_backup -name .gitconfig")
	if code != 0 || strings.TrimSpace(findOut) == "" {
		t.Fatalf("expected a backup of the old .gitconfig under %s/.ten_backup, find output: %q", home, findOut)
	}
	content := sb.ReadFile(t, strings.TrimSpace(findOut))
	if content != "old local config\n" {
		t.Fatalf("unexpected backup content: %q", content)
	}
}

func TestApply_SecondRunIsIdempotent(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "[user]\n\tname = Taro\n")

	if _, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("first apply failed")
	}
	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("second apply failed (exit %d): %s", code, out)
	}
	if strings.Contains(out, "create symlink") {
		t.Fatalf("expected no-op on second apply, got output: %s", out)
	}

	findOut, _, _ := sb.Exec(t, "find "+home+"/.ten_backup -type f 2>/dev/null")
	if strings.TrimSpace(findOut) != "" {
		t.Fatalf("expected no backup from idempotent second apply, found: %q", findOut)
	}
}

func TestApply_ErrorsWhenLinkSourceDoesNotExist(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	// The tool points at a file that isn't in the dotfiles repo at all.
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)

	out, code := sb.Run(t, home, "apply")
	if code == 0 {
		t.Fatalf("expected apply to fail on a missing link source, got exit 0: %s", out)
	}
	if !strings.Contains(out, home+"/dotfiles/git/.gitconfig") {
		t.Fatalf("expected the error to name the missing source, got: %s", out)
	}
	if _, _, ok := sb.Lstat(t, home+"/.gitconfig"); ok {
		t.Fatalf("expected no (dangling) symlink to be created for a missing source")
	}
}

func TestApply_DryRunMakesNoChanges(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "[user]\n\tname = Taro\n")

	out, code := sb.Run(t, home, "apply", "--dry-run")
	if code != 0 {
		t.Fatalf("ten apply --dry-run failed (exit %d): %s", code, out)
	}
	if !strings.Contains(out, "dry-run") {
		t.Fatalf("expected dry-run output to say so, got: %s", out)
	}

	_, _, ok := sb.Lstat(t, home+"/.gitconfig")
	if ok {
		t.Fatalf("expected no symlink to be created under --dry-run")
	}
}

func TestApply_MultiToolDAGOrder(t *testing.T) {
	sb := tencli.NewSandbox(t)
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
	sb := tencli.NewSandbox(t)
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
	sb := tencli.NewSandbox(t)
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
	sb := tencli.NewSandbox(t)
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
	sb := tencli.NewSandbox(t)
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
	sb := tencli.NewSandbox(t)
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
	sb := tencli.NewSandbox(t)
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

func TestApply_ErrorsWhenDotfilesRootIsUnset(t *testing.T) {
	sb := tencli.NewSandbox(t)
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
	sb := tencli.NewSandbox(t)
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
	sb := tencli.NewSandbox(t)
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
	sb := tencli.NewSandbox(t)
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
	sb := tencli.NewSandbox(t)
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

func TestApply_ReportsWhatWasAppliedBeforeAFailure(t *testing.T) {
	sb := tencli.NewSandbox(t)
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
