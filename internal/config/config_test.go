package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func TestLoadLocal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ten.local.toml")
	writeFile(t, path, `
[core]
dotfiles_root = "~/dotfiles"
profile = "work"

[vars]
git_email = "taro@example.com"

[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig.local" }
`)

	got, err := LoadLocal(path)
	if err != nil {
		t.Fatalf("LoadLocal: %v", err)
	}
	if got.Core.DotfilesRoot != "~/dotfiles" || got.Core.Profile != "work" {
		t.Fatalf("unexpected core: %+v", got.Core)
	}
	if got.Vars["git_email"] != "taro@example.com" {
		t.Fatalf("unexpected vars: %+v", got.Vars)
	}
	want := Tool{Links: map[string]string{"home:.gitconfig": "git/.gitconfig.local"}}
	if !reflect.DeepEqual(got.Tools["git"], want) {
		t.Fatalf("unexpected tools[git]: %+v", got.Tools["git"])
	}
}

func TestLoadRepo_MissingFileIsOptional(t *testing.T) {
	dir := t.TempDir()
	repo, ok, err := LoadRepo(filepath.Join(dir, "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false for missing file")
	}
	if len(repo.Tools) != 0 {
		t.Fatalf("expected empty Repo, got %+v", repo)
	}
}

func TestMerge_LocalOverridesRepoWholeTool(t *testing.T) {
	repo := Repo{Tools: map[string]Tool{
		"git":  {Links: map[string]string{"home:.gitconfig": "git/.gitconfig"}, PostApply: "echo repo"},
		"nvim": {Links: map[string]string{"xdg:nvim": "nvim"}},
	}}
	local := Local{
		Core: Core{DotfilesRoot: "~/dotfiles"},
		Vars: map[string]string{"k": "v"},
		Tools: map[string]Tool{
			"git": {Links: map[string]string{"home:.gitconfig": "git/.gitconfig.local"}},
		},
	}

	got := Merge(repo, local)

	if got.DotfilesRoot != "~/dotfiles" {
		t.Fatalf("unexpected DotfilesRoot: %q", got.DotfilesRoot)
	}
	wantGit := Tool{Links: map[string]string{"home:.gitconfig": "git/.gitconfig.local"}}
	if !reflect.DeepEqual(got.Tools["git"], wantGit) {
		t.Fatalf("expected whole-tool replace, got %+v", got.Tools["git"])
	}
	if _, ok := got.Tools["nvim"]; !ok {
		t.Fatalf("expected nvim to survive merge untouched")
	}
}
