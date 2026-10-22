package e2e_test

import (
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/testutil/tencli"
)

func TestRoot_AddListRemove(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()
	sb.Init(t, home, home+"/dotfiles")
	sb.Exec(t, "mkdir -p "+home+"/work-dotfiles")

	out, code := sb.Run(t, home, "root", "add", "work", home+"/work-dotfiles")
	if code != 0 {
		t.Fatalf("ten root add failed (exit %d): %s", code, out)
	}

	out, code = sb.Run(t, home, "root", "list")
	if code != 0 || !strings.Contains(out, "work") || !strings.Contains(out, home+"/work-dotfiles") {
		t.Fatalf("expected root list to show the registered root, got (exit %d): %s", code, out)
	}

	out, code = sb.Run(t, home, "root", "remove", "work")
	if code != 0 {
		t.Fatalf("ten root remove failed (exit %d): %s", code, out)
	}
	out, code = sb.Run(t, home, "root", "list")
	if code != 0 || strings.Contains(out, "work") {
		t.Fatalf("expected work root to be gone after remove, got (exit %d): %s", code, out)
	}
}

func TestRoot_AddErrorsWhenPathDoesNotExist(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()
	sb.Init(t, home, home+"/dotfiles")

	_, code := sb.Run(t, home, "root", "add", "work", home+"/does-not-exist")
	if code == 0 {
		t.Fatalf("expected ten root add to fail for a nonexistent path")
	}
}

func TestApply_UsesExternalRootForLinksAndHookWorkingDirectory(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "personal\n")

	sb.WriteFile(t, home+"/work-dotfiles/ten.toml", `
[tools.zsh-work]
links = { "home:.zshrc.d/work.zsh" = "zsh/work.zsh" }
once  = "cat relative-marker.txt > `+home+`/once-output.txt"
`)
	sb.WriteFile(t, home+"/work-dotfiles/zsh/work.zsh", "export WORK=1\n")
	sb.WriteFile(t, home+"/work-dotfiles/relative-marker.txt", "from-work-root\n")

	out, code := sb.Run(t, home, "root", "add", "work", home+"/work-dotfiles")
	if code != 0 {
		t.Fatalf("ten root add failed (exit %d): %s", code, out)
	}

	out, code = sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("ten apply failed (exit %d): %s", code, out)
	}

	isLink, target, ok := sb.Lstat(t, home+"/.zshrc.d/work.zsh")
	if !ok || !isLink || target != home+"/work-dotfiles/zsh/work.zsh" {
		t.Fatalf("expected .zshrc.d/work.zsh to symlink into the external root, got isLink=%v target=%q ok=%v", isLink, target, ok)
	}
	isLinkGit, targetGit, okGit := sb.Lstat(t, home+"/.gitconfig")
	if !okGit || !isLinkGit || targetGit != home+"/dotfiles/git/.gitconfig" {
		t.Fatalf("expected the personal tool's link to still resolve against dotfiles_root, got isLink=%v target=%q ok=%v", isLinkGit, targetGit, okGit)
	}
	if got := sb.ReadFile(t, home+"/once-output.txt"); got != "from-work-root\n" {
		t.Fatalf("expected the once hook to run with the external root as its working directory, got %q", got)
	}
}
