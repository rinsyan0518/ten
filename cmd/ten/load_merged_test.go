package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rinsyan0518/ten/internal/state"
)

func writeTenFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func TestLoadMerged_BaseProfileLocalOnly(t *testing.T) {
	dotfilesRoot := t.TempDir()
	writeTenFile(t, filepath.Join(dotfilesRoot, "ten.toml"), `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	writeTenFile(t, filepath.Join(dotfilesRoot, "ten.work.toml"), `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig.work" }
`)
	writeTenFile(t, filepath.Join(dotfilesRoot, "ten.local.toml"), `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig.local" }
`)

	merged, repoFound, err := loadMerged(dotfilesRoot, "work", nil)
	if err != nil {
		t.Fatalf("loadMerged: %v", err)
	}
	if !repoFound {
		t.Fatalf("expected repoFound=true")
	}
	if got := merged.Tools["git"].Links["home:.gitconfig"]; got != "git/.gitconfig.local" {
		t.Fatalf("expected local to win, got %q", got)
	}
	if merged.DotfilesRoot != dotfilesRoot {
		t.Fatalf("DotfilesRoot = %q, want %q", merged.DotfilesRoot, dotfilesRoot)
	}
}

func TestLoadMerged_ExternalRootMissingDirectoryErrors(t *testing.T) {
	dotfilesRoot := t.TempDir()
	writeTenFile(t, filepath.Join(dotfilesRoot, "ten.toml"), `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	missingRoot := filepath.Join(dotfilesRoot, "does-not-exist-root")

	_, _, err := loadMerged(dotfilesRoot, "", []state.ExternalRoot{{Name: "work", Path: missingRoot}})
	if err == nil {
		t.Fatalf("expected an error when a registered external root's directory no longer exists")
	}
}

func TestLoadMerged_ExternalRootWithoutTenTomlContributesNothing(t *testing.T) {
	dotfilesRoot := t.TempDir()
	writeTenFile(t, filepath.Join(dotfilesRoot, "ten.toml"), `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	emptyExternalRoot := t.TempDir()

	merged, repoFound, err := loadMerged(dotfilesRoot, "", []state.ExternalRoot{{Name: "work", Path: emptyExternalRoot}})
	if err != nil {
		t.Fatalf("loadMerged: %v", err)
	}
	if !repoFound {
		t.Fatalf("expected repoFound=true from the base ten.toml")
	}
	if _, ok := merged.Tools["git"]; !ok {
		t.Fatalf("expected base tool to survive an external root with no ten.toml, got %+v", merged.Tools)
	}
}

func TestLoadMerged_ExternalRootToolLinksRootPointsAtExternalRoot(t *testing.T) {
	dotfilesRoot := t.TempDir()
	writeTenFile(t, filepath.Join(dotfilesRoot, "ten.toml"), `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	workRoot := t.TempDir()
	writeTenFile(t, filepath.Join(workRoot, "ten.toml"), `
[tools.zsh-work]
links = { "home:.zshrc.d/work.zsh" = "zsh/work.zsh" }
`)

	merged, _, err := loadMerged(dotfilesRoot, "", []state.ExternalRoot{{Name: "work", Path: workRoot}})
	if err != nil {
		t.Fatalf("loadMerged: %v", err)
	}
	if merged.LinksRoot["zsh-work"] != workRoot {
		t.Fatalf("LinksRoot[zsh-work] = %q, want %q", merged.LinksRoot["zsh-work"], workRoot)
	}
}

func TestLoadMerged_MultipleExternalRootsMergeInRegistrationOrder(t *testing.T) {
	dotfilesRoot := t.TempDir()
	writeTenFile(t, filepath.Join(dotfilesRoot, "ten.toml"), "")
	rootA := t.TempDir()
	writeTenFile(t, filepath.Join(rootA, "ten.toml"), `
[tools.zsh-work]
links = { "home:.zshrc" = "a.zsh" }
`)
	rootB := t.TempDir()
	writeTenFile(t, filepath.Join(rootB, "ten.toml"), `
[tools.zsh-work]
links = { "home:.zshrc" = "b.zsh" }
`)

	merged, _, err := loadMerged(dotfilesRoot, "", []state.ExternalRoot{
		{Name: "a", Path: rootA},
		{Name: "b", Path: rootB},
	})
	if err != nil {
		t.Fatalf("loadMerged: %v", err)
	}
	if got := merged.Tools["zsh-work"].Links["home:.zshrc"]; got != "b.zsh" {
		t.Fatalf("expected the later-registered root to win, got %q", got)
	}
	if merged.LinksRoot["zsh-work"] != rootB {
		t.Fatalf("expected LinksRoot to follow the winning root, got %q", merged.LinksRoot["zsh-work"])
	}
}
