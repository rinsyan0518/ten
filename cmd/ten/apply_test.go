package main

import (
	"path/filepath"
	"testing"

	"github.com/rinsyan0518/ten/internal/state"
)

// TestApply_PreservesExternalRootsAcrossRepeatedApplies is a regression
// test for a bug where `ten apply` saved a newState built from scratch
// (apply.Execute only populates ManagedResources) without carrying
// ExternalRoots over from the state it loaded — so every `ten apply`
// silently wiped ExternalRoots from ten.state.json. It seeds
// ten.state.json directly (not via `ten root add`) so the first apply
// in this test already starts from a state with ExternalRoots set, the
// same shape `ten root add` itself would have produced.
func TestApply_PreservesExternalRootsAcrossRepeatedApplies(t *testing.T) {
	home := newInitTestHome(t)
	dotfilesRoot := t.TempDir()
	writeTenFile(t, filepath.Join(dotfilesRoot, "ten.toml"), "")
	externalRoot := t.TempDir()
	writeTenFile(t, filepath.Join(externalRoot, "ten.toml"), "")

	statePath := filepath.Join(home, ".local", "state", "ten", "ten.state.json")
	seeded := state.State{
		DotfilesRoot:     dotfilesRoot,
		ExternalRoots:    []state.ExternalRoot{{Name: "work", Path: externalRoot}},
		ManagedResources: map[string]state.Resource{},
	}
	if err := state.Save(statePath, seeded); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	if _, err := runTenCmd(t, "apply"); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	afterFirst := loadState(t, home).ExternalRoots
	if len(afterFirst) != 1 || afterFirst[0].Name != "work" {
		t.Fatalf("expected ExternalRoots to survive the first apply, got %+v", afterFirst)
	}

	if _, err := runTenCmd(t, "apply"); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	afterSecond := loadState(t, home).ExternalRoots
	if len(afterSecond) != 1 || afterSecond[0].Name != "work" {
		t.Fatalf("expected ExternalRoots to survive the second apply too, got %+v", afterSecond)
	}
}

// TestApply_KeepsExternalRootManagedResourceAfterTwoApplies exercises
// the full failure mode of the same bug: registering an external root
// via the real `ten root add` command (not by seeding state directly),
// then running `ten apply` twice. Under the bug, the first apply wipes
// ExternalRoots from ten.state.json even though the external root's
// resource was created; the second apply then can no longer load the
// external root's ten.toml, treats its tool as no longer desired, and
// prunes the managed resource it just created.
func TestApply_KeepsExternalRootManagedResourceAfterTwoApplies(t *testing.T) {
	home := newInitTestHome(t)
	dotfilesRoot := t.TempDir()
	writeTenFile(t, filepath.Join(dotfilesRoot, "ten.toml"), "")

	externalRoot := t.TempDir()
	writeTenFile(t, filepath.Join(externalRoot, "ten.toml"), `
[tools.zsh-work]
links = { "home:.zshrc.d/work.zsh" = "zsh/work.zsh" }
`)
	writeTenFile(t, filepath.Join(externalRoot, "zsh", "work.zsh"), "# work zsh config\n")

	if _, err := runTenCmd(t, "init", "--path", dotfilesRoot); err != nil {
		t.Fatalf("execute init: %v", err)
	}
	if _, err := runTenCmd(t, "root", "add", "work", externalRoot); err != nil {
		t.Fatalf("execute root add: %v", err)
	}

	if _, err := runTenCmd(t, "apply"); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	midway := loadState(t, home)
	if len(midway.ExternalRoots) != 1 {
		t.Fatalf("expected ExternalRoots to survive the first apply, got %+v", midway.ExternalRoots)
	}
	if _, ok := midway.ManagedResources[filepath.Join(home, ".zshrc.d", "work.zsh")]; !ok {
		t.Fatalf("expected the external root's resource to be tracked after the first apply, got %+v", midway.ManagedResources)
	}

	if _, err := runTenCmd(t, "apply"); err != nil {
		t.Fatalf("second apply: %v", err)
	}

	final := loadState(t, home)
	if len(final.ExternalRoots) != 1 {
		t.Fatalf("expected ExternalRoots to survive the second apply, got %+v", final.ExternalRoots)
	}
	if _, ok := final.ManagedResources[filepath.Join(home, ".zshrc.d", "work.zsh")]; !ok {
		t.Fatalf("expected the external root's resource to still be tracked after the second apply, got %+v", final.ManagedResources)
	}

	target := filepath.Join(home, ".zshrc.d", "work.zsh")
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatalf("expected %s to still be a valid symlink after the second apply: %v", target, err)
	}
	wantTarget, err := filepath.EvalSymlinks(filepath.Join(externalRoot, "zsh", "work.zsh"))
	if err != nil {
		t.Fatalf("resolve expected symlink target: %v", err)
	}
	if resolved != wantTarget {
		t.Fatalf("symlink target = %q, want %q", resolved, wantTarget)
	}
}
