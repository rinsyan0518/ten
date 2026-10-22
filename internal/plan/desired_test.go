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

func TestDesired_UsesLinksRootAndTemplatesRootWhenSet(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"zsh-work": {
				Links:     map[string]string{"home:.zshrc.d/work.zsh": "zsh/work.zsh"},
				Templates: map[string]string{"home:.zshrc.d/work.local": "zsh/work.local.tmpl"},
			},
		},
		LinksRoot:     map[string]string{"zsh-work": "/work-root"},
		TemplatesRoot: map[string]string{"zsh-work": "/work-root"},
	}

	got, err := plan.Desired(merged, []string{"zsh-work"}, pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []plan.Target{
		{Tool: "zsh-work", Kind: "symlink", Target: "/home/taro/.zshrc.d/work.zsh", Source: "/work-root/zsh/work.zsh"},
		{Tool: "zsh-work", Kind: "template", Target: "/home/taro/.zshrc.d/work.local", Source: "/work-root/zsh/work.local.tmpl"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestDesired_FallsBackToDotfilesRootWhenLinksRootUnset(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git": {Links: map[string]string{"home:.gitconfig": "git/.gitconfig"}},
		},
	}

	got, err := plan.Desired(merged, []string{"git"}, pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []plan.Target{{Tool: "git", Kind: "symlink", Target: "/home/taro/.gitconfig", Source: "/dotfiles/git/.gitconfig"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestDesired_ErrorsWhenTwoToolsFromDifferentRootsClaimTheSameTarget(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"zsh":      {Links: map[string]string{"home:.zshrc": "zsh/.zshrc"}},
			"zsh-work": {Links: map[string]string{"home:.zshrc": "zsh/.zshrc.work"}},
		},
		LinksRoot: map[string]string{"zsh-work": "/work-root"},
	}

	_, err := plan.Desired(merged, []string{"zsh", "zsh-work"}, pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"})
	if err == nil {
		t.Fatalf("expected error for a target claimed by tools from two different roots")
	}
}
