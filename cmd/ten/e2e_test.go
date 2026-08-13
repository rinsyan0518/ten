package main_test

import (
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/dockertest"
)

func TestApply_MultiToolDAGOrder(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.WriteFile(t, home+"/.config/ten/ten.local.toml", `
[core]
dotfiles_root = "`+home+`/dotfiles"
profile = "work"
`)
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
enabled_tools = ["git"]

[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }

[tools.git-work]
depends_on = ["git"]
links = { "home:.gitconfig.local" = "git/.gitconfig.work" }
`)
	sb.WriteFile(t, home+"/dotfiles/ten.work.toml", `
enabled_tools = ["git-work"]
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

	sb.WriteFile(t, home+"/.config/ten/ten.local.toml", `
[core]
dotfiles_root = "`+home+`/dotfiles"
`)
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
enabled_tools = ["git"]

[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }

[tools.nvim]
links = { "xdg:nvim" = "nvim" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("ten apply failed (exit %d): %s", code, out)
	}
	if _, _, ok := sb.Lstat(t, home+"/.config/nvim"); ok {
		t.Fatalf("expected nvim to NOT be applied since it's not in enabled_tools")
	}
}

func TestApply_GroupedPlanOutputFormat(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.WriteFile(t, home+"/.config/ten/ten.local.toml", `
[core]
dotfiles_root = "`+home+`/dotfiles"
`)
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

	sb.WriteFile(t, home+"/.config/ten/ten.local.toml", `
[core]
dotfiles_root = "`+home+`/dotfiles"
`)
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
}

func TestApply_ConvertingLinkToTemplateDoesNotWriteThroughSymlink(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.WriteFile(t, home+"/.config/ten/ten.local.toml", `
[core]
dotfiles_root = "`+home+`/dotfiles"

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

func TestApply_FailFastRetainsUntouchedResourcesInState(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.WriteFile(t, home+"/.config/ten/ten.local.toml", `
[core]
dotfiles_root = "`+home+`/dotfiles"
`)
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
	stateJSON := sb.ReadFile(t, home+"/.config/ten/ten.state.json")
	if !strings.Contains(stateJSON, home+"/.config/nvim") {
		t.Fatalf("expected state to still track nvim's target after fail-fast, got: %s", stateJSON)
	}
}

func TestApply_HookFailureStopsLaterToolsFailFast(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.WriteFile(t, home+"/.config/ten/ten.local.toml", `
[core]
dotfiles_root = "`+home+`/dotfiles"
`)
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

func TestDestroy_RemovesManagedSymlink(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.WriteFile(t, home+"/.config/ten/ten.local.toml", `
[core]
dotfiles_root = "`+home+`/dotfiles"
`)
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
}

func TestDestroy_RestoresBackup(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.WriteFile(t, home+"/.config/ten/ten.local.toml", `
[core]
dotfiles_root = "`+home+`/dotfiles"
`)
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

	sb.WriteFile(t, home+"/.config/ten/ten.local.toml", `
[core]
dotfiles_root = "`+home+`/dotfiles"
`)
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

	sb.WriteFile(t, home+"/.config/ten/ten.local.toml", `
[core]
dotfiles_root = "`+home+`/dotfiles"
`)
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

	sb.WriteFile(t, home+"/.config/ten/ten.local.toml", `
[core]
dotfiles_root = "`+home+`/dotfiles"
`)
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
	// through the run. Destroy order is the reverse of apply order
	// (c, b, a): c is destroyed successfully first, b's restore then
	// fails, and a is never reached.
	stateJSON := sb.ReadFile(t, home+"/.config/ten/ten.state.json")
	sabotaged := strings.Replace(stateJSON,
		`"source": "`+home+`/dotfiles/b/.b"`,
		`"source": "`+home+`/dotfiles/b/.b",
      "backup_path": "`+home+`/.ten_backup/nonexistent/bogus"`,
		1)
	if sabotaged == stateJSON {
		t.Fatalf("failed to sabotage state.json: source line for b not found in %s", stateJSON)
	}
	// WriteFile's heredoc requires a trailing newline to terminate cleanly.
	sb.WriteFile(t, home+"/.config/ten/ten.state.json", sabotaged+"\n")

	out, code := sb.Run(t, home, "destroy")
	if code == 0 {
		t.Fatalf("expected destroy to fail, got exit 0: %s", out)
	}

	if _, _, ok := sb.Lstat(t, home+"/.c"); ok {
		t.Fatalf("expected .c to be removed before the failure")
	}
	if isLink, _, ok := sb.Lstat(t, home+"/.b"); !ok || !isLink {
		t.Fatalf("expected .b to remain a symlink after its failed restore")
	}
	if isLink, _, ok := sb.Lstat(t, home+"/.a"); !ok || !isLink {
		t.Fatalf("expected .a to remain untouched (not yet reached)")
	}

	finalState := sb.ReadFile(t, home+"/.config/ten/ten.state.json")
	if strings.Contains(finalState, home+"/.c\"") {
		t.Fatalf("expected .c to be dropped from state after successful removal, got: %s", finalState)
	}
	if !strings.Contains(finalState, home+"/.b\"") {
		t.Fatalf("expected .b to still be tracked in state after its failed removal, got: %s", finalState)
	}
	if !strings.Contains(finalState, home+"/.a\"") {
		t.Fatalf("expected .a to still be tracked in state (never reached), got: %s", finalState)
	}
}

func TestDestroy_DryRunMakesNoChanges(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.WriteFile(t, home+"/.config/ten/ten.local.toml", `
[core]
dotfiles_root = "`+home+`/dotfiles"
`)
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
