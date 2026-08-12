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
