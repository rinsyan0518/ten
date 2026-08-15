package main_test

import (
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/dockertest"
)

func TestApply_CreatesSymlinkForSingleTool(t *testing.T) {
	sb := dockertest.NewSandbox(t)
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
	sb := dockertest.NewSandbox(t)
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
	sb := dockertest.NewSandbox(t)
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
	sb := dockertest.NewSandbox(t)
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
	sb := dockertest.NewSandbox(t)
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
