package main

import (
	"path/filepath"
	"testing"

	"github.com/rinsyan0518/ten/internal/pathresolve"
	"github.com/rinsyan0518/ten/internal/state"
)

// testEnv builds the explicit Env for a test home; loadBootstrap takes
// resolved values, so no environment variables need to be pinned.
func testEnv(home string) pathresolve.Env {
	return pathresolve.Env{
		Home:          home,
		XDGConfigHome: filepath.Join(home, ".config"),
		XDGStateHome:  filepath.Join(home, ".local", "state"),
	}
}

func TestLoadBootstrap_ErrorsWhenDotfilesRootUnset(t *testing.T) {
	home := t.TempDir()

	if _, _, err := loadBootstrap(testEnv(home)); err == nil {
		t.Fatalf("expected an error when ten.state.json has no dotfiles_root")
	}
}

func TestLoadBootstrap_ReturnsStateWhenDotfilesRootSet(t *testing.T) {
	home := t.TempDir()
	statePath := filepath.Join(home, ".local", "state", "ten", "ten.state.json")
	want := state.State{DotfilesRoot: "/home/taro/dotfiles", Profile: "work", ManagedResources: map[string]state.Resource{}}
	if err := state.Save(statePath, want); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	got, gotPath, err := loadBootstrap(testEnv(home))
	if err != nil {
		t.Fatalf("loadBootstrap: %v", err)
	}
	if got.DotfilesRoot != want.DotfilesRoot || got.Profile != want.Profile {
		t.Fatalf("unexpected bootstrap state: %+v", got)
	}
	if gotPath != statePath {
		t.Fatalf("statePath mismatch: got %q want %q", gotPath, statePath)
	}
}
