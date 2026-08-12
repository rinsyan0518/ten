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
