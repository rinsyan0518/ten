package apply_test

import (
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/dockertest"
)

func TestApply_CreatesSymlinkForSingleTool(t *testing.T) {
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
