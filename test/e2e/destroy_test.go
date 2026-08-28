package e2e_test

import (
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/testutil/tencli"
)

func TestDestroy_SkipsTargetUserReplacedWithTheirOwnFile(t *testing.T) {
	sb := tencli.NewSandbox(t)
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

func TestDestroy_NoOpsWhenDotfilesRootIsGoneAndNothingIsManaged(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.Exec(t, "rm -rf "+home+"/dotfiles")

	out, code := sb.Run(t, home, "destroy")
	if code != 0 {
		t.Fatalf("expected destroy to no-op successfully when nothing is managed, even with dotfiles_root gone, got exit %d: %s", code, out)
	}
}

func TestDestroy_RemovesManagedSymlink(t *testing.T) {
	sb := tencli.NewSandbox(t)
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
	sb := tencli.NewSandbox(t)
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
	sb := tencli.NewSandbox(t)
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
	sb := tencli.NewSandbox(t)
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
	sb := tencli.NewSandbox(t)
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
	sb := tencli.NewSandbox(t)
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
      "backup_path": "`+home+`/.local/state/ten/backup/nonexistent/bogus"`,
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
	if !strings.Contains(out, home+"/.gitconfig <- "+home+"/.local/state/ten/backup/") {
		t.Fatalf("expected the restore line to name the backup it comes from, got: %s", out)
	}
}

func TestDestroy_DryRunMakesNoChanges(t *testing.T) {
	sb := tencli.NewSandbox(t)
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

func TestDestroy_SkipsTemplateOutputEditedByTheUser(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
templates = { "home:.gitconfig.local" = "git/tmpl" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/tmpl", "managed by ten\n")

	if out, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("apply failed (exit %d): %s", code, out)
	}

	// The user takes the rendered file over; destroy must refuse to
	// delete what is no longer ten's own output.
	sb.WriteFile(t, home+"/.gitconfig.local", "my own edits\n")

	out, code := sb.Run(t, home, "destroy")
	if code != 0 {
		t.Fatalf("destroy failed (exit %d): %s", code, out)
	}
	if !strings.Contains(out, "content changed") {
		t.Fatalf("expected a warning naming the reason, got: %s", out)
	}
	if got := sb.ReadFile(t, home+"/.gitconfig.local"); got != "my own edits\n" {
		t.Fatalf("the edited file must survive destroy, got %q", got)
	}
}

func TestDestroy_RestoreLeavesNoEmptyBackupSkeleton(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "repo version\n")
	sb.WriteFile(t, home+"/.gitconfig", "original\n")

	if out, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("apply failed (exit %d): %s", code, out)
	}
	if out, code := sb.Run(t, home, "destroy"); code != 0 {
		t.Fatalf("destroy failed (exit %d): %s", code, out)
	}

	if got := sb.ReadFile(t, home+"/.gitconfig"); got != "original\n" {
		t.Fatalf("expected the backup restored, got %q", got)
	}
	// The restore emptied the only backup; the whole backup tree
	// must be gone instead of accumulating empty timestamp skeletons.
	if _, _, ok := sb.Lstat(t, home+"/.local/state/ten/backup"); ok {
		out, _, _ := sb.Exec(t, "find "+home+"/.local/state/ten/backup")
		t.Fatalf("expected the emptied backup root to be removed, found:\n%s", out)
	}
}
