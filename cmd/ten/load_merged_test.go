package main

import (
	"os"
	"path/filepath"
	"testing"
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

	merged, repoFound, err := loadMerged(dotfilesRoot, "work")
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
