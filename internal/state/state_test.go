package state_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/rinsyan0518/ten/internal/state"
)

func TestLoad_MissingFileReturnsEmptyState(t *testing.T) {
	dir := t.TempDir()
	got, err := state.Load(filepath.Join(dir, "ten.state.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.ManagedResources) != 0 {
		t.Fatalf("expected empty state, got %+v", got)
	}
}

func TestSaveThenLoad_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ten", "ten.state.json")

	want := state.State{
		LastApplied: time.Date(2026, 8, 11, 22, 44, 5, 0, time.UTC),
		ManagedResources: map[string]state.Resource{
			"/home/taro/.config/nvim": {
				Tool:   "nvim",
				Type:   "symlink",
				Source: "/home/taro/dotfiles/nvim",
			},
		},
	}

	if err := state.Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := state.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !got.LastApplied.Equal(want.LastApplied) {
		t.Fatalf("LastApplied mismatch: got %v want %v", got.LastApplied, want.LastApplied)
	}
	if got.ManagedResources["/home/taro/.config/nvim"] != want.ManagedResources["/home/taro/.config/nvim"] {
		t.Fatalf("resource mismatch: got %+v", got.ManagedResources)
	}
}
