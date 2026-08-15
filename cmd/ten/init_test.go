package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/rinsyan0518/ten/internal/state"
)

func loadState(t *testing.T, home string) state.State {
	t.Helper()
	got, err := state.Load(filepath.Join(home, ".local", "state", "ten", "ten.state.json"))
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	return got
}

func TestInit_DefaultsPathToCwd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// pathresolve.XDGStateHome checks $XDG_STATE_HOME before falling back
	// to a home-derived default, so pin it explicitly — otherwise this
	// test's isolation depends on the host shell not exporting
	// XDG_STATE_HOME, which is not a safe assumption (this project is
	// itself a dotfiles manager; a dev machine bootstrapped with it would
	// have XDG_STATE_HOME set globally, diverging from `home`).
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	dotfilesRoot := t.TempDir()
	t.Chdir(dotfilesRoot)

	cmd := newRootCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute init: %v", err)
	}

	// os.Getwd() (used internally by runInit when --path is omitted) may
	// return the symlink-resolved form of dotfilesRoot — e.g. on macOS,
	// where t.TempDir() paths live under /var, itself a symlink to
	// /private/var — so resolve both sides before comparing.
	wantRoot, err := filepath.EvalSymlinks(dotfilesRoot)
	if err != nil {
		t.Fatalf("resolve expected dotfiles root: %v", err)
	}
	gotRoot, err := filepath.EvalSymlinks(loadState(t, home).DotfilesRoot)
	if err != nil {
		t.Fatalf("resolve got dotfiles root: %v", err)
	}
	if gotRoot != wantRoot {
		t.Fatalf("expected DotfilesRoot %q, got %q", wantRoot, gotRoot)
	}
}

func TestInit_PathFlagOverridesCwd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// pathresolve.XDGStateHome checks $XDG_STATE_HOME before falling back
	// to a home-derived default, so pin it explicitly — otherwise this
	// test's isolation depends on the host shell not exporting
	// XDG_STATE_HOME, which is not a safe assumption (this project is
	// itself a dotfiles manager; a dev machine bootstrapped with it would
	// have XDG_STATE_HOME set globally, diverging from `home`).
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	dotfilesRoot := t.TempDir()

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"init", "--path", dotfilesRoot})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute init: %v", err)
	}

	got := loadState(t, home)
	if got.DotfilesRoot != dotfilesRoot {
		t.Fatalf("expected DotfilesRoot %q, got %q", dotfilesRoot, got.DotfilesRoot)
	}
}

func TestInit_ProfileFlagSetsProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// pathresolve.XDGStateHome checks $XDG_STATE_HOME before falling back
	// to a home-derived default, so pin it explicitly — otherwise this
	// test's isolation depends on the host shell not exporting
	// XDG_STATE_HOME, which is not a safe assumption (this project is
	// itself a dotfiles manager; a dev machine bootstrapped with it would
	// have XDG_STATE_HOME set globally, diverging from `home`).
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	dotfilesRoot := t.TempDir()

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"init", "--path", dotfilesRoot, "--profile", "work"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute init: %v", err)
	}

	if got := loadState(t, home).Profile; got != "work" {
		t.Fatalf("expected Profile %q, got %q", "work", got)
	}
}

func TestInit_OmittingProfilePreservesExisting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// pathresolve.XDGStateHome checks $XDG_STATE_HOME before falling back
	// to a home-derived default, so pin it explicitly — otherwise this
	// test's isolation depends on the host shell not exporting
	// XDG_STATE_HOME, which is not a safe assumption (this project is
	// itself a dotfiles manager; a dev machine bootstrapped with it would
	// have XDG_STATE_HOME set globally, diverging from `home`).
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	dotfilesRoot := t.TempDir()

	first := newRootCmd()
	first.SetOut(&bytes.Buffer{})
	first.SetArgs([]string{"init", "--path", dotfilesRoot, "--profile", "work"})
	if err := first.Execute(); err != nil {
		t.Fatalf("execute first init: %v", err)
	}

	second := newRootCmd()
	second.SetOut(&bytes.Buffer{})
	second.SetArgs([]string{"init", "--path", dotfilesRoot})
	if err := second.Execute(); err != nil {
		t.Fatalf("execute second init: %v", err)
	}

	if got := loadState(t, home).Profile; got != "work" {
		t.Fatalf("expected Profile to stay %q after re-running init without --profile, got %q", "work", got)
	}
}

func TestInit_PreservesManagedResources(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// pathresolve.XDGStateHome checks $XDG_STATE_HOME before falling back
	// to a home-derived default, so pin it explicitly — otherwise this
	// test's isolation depends on the host shell not exporting
	// XDG_STATE_HOME, which is not a safe assumption (this project is
	// itself a dotfiles manager; a dev machine bootstrapped with it would
	// have XDG_STATE_HOME set globally, diverging from `home`).
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	dotfilesRoot := t.TempDir()
	statePath := filepath.Join(home, ".local", "state", "ten", "ten.state.json")
	seeded := state.State{
		ManagedResources: map[string]state.Resource{
			"/home/taro/.gitconfig": {Tool: "git", Type: "symlink", Source: "/home/taro/dotfiles/git/.gitconfig"},
		},
	}
	if err := state.Save(statePath, seeded); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"init", "--path", dotfilesRoot})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute init: %v", err)
	}

	got := loadState(t, home)
	if len(got.ManagedResources) != 1 {
		t.Fatalf("expected init to preserve managed_resources untouched, got %+v", got.ManagedResources)
	}
}

func TestInit_ErrorsWhenPathDoesNotExist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// pathresolve.XDGStateHome checks $XDG_STATE_HOME before falling back
	// to a home-derived default, so pin it explicitly — otherwise this
	// test's isolation depends on the host shell not exporting
	// XDG_STATE_HOME, which is not a safe assumption (this project is
	// itself a dotfiles manager; a dev machine bootstrapped with it would
	// have XDG_STATE_HOME set globally, diverging from `home`).
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"init", "--path", filepath.Join(home, "does-not-exist")})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected init to fail for a nonexistent --path")
	}
}
