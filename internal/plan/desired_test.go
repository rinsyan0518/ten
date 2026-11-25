package plan_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/config"
	"github.com/rinsyan0518/ten/internal/pathresolve"
	"github.com/rinsyan0518/ten/internal/plan"
)

func TestDesired_ResolvesLinksAndTemplatesInSortedKeyOrder(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git": {
				Links:     map[string]string{"home:.gitconfig": "git/.gitconfig", "home:.gitignore": "git/.gitignore"},
				Templates: map[string]string{"home:.gitconfig.local": "git/gitconfig.local.tmpl"},
			},
		},
	}

	got, err := plan.Desired(merged, []string{"git"}, pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []plan.Target{
		{Tool: "git", Kind: "symlink", Target: "/home/taro/.gitconfig", Source: "/dotfiles/git/.gitconfig"},
		{Tool: "git", Kind: "symlink", Target: "/home/taro/.gitignore", Source: "/dotfiles/git/.gitignore"},
		{Tool: "git", Kind: "template", Target: "/home/taro/.gitconfig.local", Source: "/dotfiles/git/gitconfig.local.tmpl"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestDesired_ErrorsWhenTwoToolsClaimTheSameTarget(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git":      {Links: map[string]string{"home:.gitconfig": "git/.gitconfig"}},
			"git-work": {Links: map[string]string{"home:.gitconfig": "git-work/.gitconfig"}},
		},
	}

	_, err := plan.Desired(merged, []string{"git", "git-work"}, pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"})
	if err == nil {
		t.Fatalf("expected error for conflicting target, got nil")
	}
	for _, want := range []string{"/home/taro/.gitconfig", "git", "git-work"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error should mention %q, got: %v", want, err)
		}
	}
}

func TestDesired_ErrorsWhenLinksAndTemplatesClaimTheSameTarget(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git": {
				Links:     map[string]string{"home:.gitconfig": "git/.gitconfig"},
				Templates: map[string]string{"home:.gitconfig": "git/gitconfig.tmpl"},
			},
		},
	}

	_, err := plan.Desired(merged, []string{"git"}, pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"})
	if err == nil {
		t.Fatalf("expected error for link/template conflict on one target, got nil")
	}
	if !strings.Contains(err.Error(), "/home/taro/.gitconfig") {
		t.Fatalf("error should mention the conflicting target, got: %v", err)
	}
}

func TestDesired_ErrorsOnUnresolvableKey(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git": {Links: map[string]string{"nope:.gitconfig": "git/.gitconfig"}},
		},
	}

	_, err := plan.Desired(merged, []string{"git"}, pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"})
	if err == nil {
		t.Fatalf("expected error for unresolvable key")
	}
}

func TestDesired_ErrorsWhenATargetIsNestedInsideAnotherTarget(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"nvim":       {Links: map[string]string{"xdg:nvim": "nvim"}},
			"nvim-local": {Templates: map[string]string{"xdg:nvim/lua/local.lua": "nvim/local.lua.tmpl"}},
		},
	}

	_, err := plan.Desired(merged, []string{"nvim", "nvim-local"}, pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"})
	if err == nil {
		t.Fatalf("expected error for a target nested inside another target, got nil")
	}
	for _, want := range []string{"/home/taro/.config/nvim/lua/local.lua", "/home/taro/.config/nvim", "nested"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error should mention %q, got: %v", want, err)
		}
	}
}

func TestDesired_SiblingTargetsWithSharedNamePrefixAreNotNested(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git": {Links: map[string]string{"home:.git": "git/dir", "home:.gitconfig": "git/.gitconfig"}},
		},
	}

	if _, err := plan.Desired(merged, []string{"git"}, pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
