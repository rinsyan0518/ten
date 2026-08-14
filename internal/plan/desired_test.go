package plan_test

import (
	"reflect"
	"testing"

	"github.com/rinsyan0518/ten/internal/config"
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

	got, err := plan.Desired(merged, []string{"git"}, "/home/taro")
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

func TestDesired_ErrorsOnUnresolvableKey(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git": {Links: map[string]string{"nope:.gitconfig": "git/.gitconfig"}},
		},
	}

	_, err := plan.Desired(merged, []string{"git"}, "/home/taro")
	if err == nil {
		t.Fatalf("expected error for unresolvable key")
	}
}
