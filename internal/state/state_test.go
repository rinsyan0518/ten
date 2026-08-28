package state_test

import (
	"os"
	"path/filepath"
	"strings"
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

func TestSave_StampsCurrentSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ten", "ten.state.json")

	if err := state.Save(path, state.State{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := state.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Version != state.CurrentVersion {
		t.Fatalf("Version = %d, want %d", got.Version, state.CurrentVersion)
	}
}

func TestLoad_AcceptsLegacyFileWithoutVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ten.state.json")
	legacy := `{"dotfiles_root": "/home/taro/dotfiles", "last_applied": "2026-08-11T22:44:05Z", "managed_resources": {}}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("seed legacy state: %v", err)
	}

	got, err := state.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Version != 0 || got.DotfilesRoot != "/home/taro/dotfiles" {
		t.Fatalf("legacy state misread: %+v", got)
	}
}

func TestLoad_RejectsNewerSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ten.state.json")
	future := `{"version": 99, "managed_resources": {}}`
	if err := os.WriteFile(path, []byte(future), 0o644); err != nil {
		t.Fatalf("seed future state: %v", err)
	}

	_, err := state.Load(path)
	if err == nil {
		t.Fatalf("expected an error for a state file from a newer ten")
	}
	if !strings.Contains(err.Error(), "newer") {
		t.Fatalf("error should explain the file comes from a newer ten, got: %v", err)
	}
}

func TestSave_ReplacesExistingFileAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ten", "ten.state.json")

	if err := state.Save(path, state.State{Profile: "one"}); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after first Save: %v", err)
	}

	if err := state.Save(path, state.State{Profile: "two"}); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after second Save: %v", err)
	}

	// A crash mid-write must never leave a truncated, unparsable
	// ten.state.json: Save has to write a fresh file and rename it into
	// place, not truncate the existing file in place. Rename semantics
	// mean the path refers to a new file afterwards.
	if os.SameFile(before, after) {
		t.Fatalf("Save truncated the existing state file in place; expected write-to-temp-then-rename")
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("read state dir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != filepath.Base(path) {
			t.Fatalf("Save left extra file %q in state dir", e.Name())
		}
	}

	got, err := state.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Profile != "two" {
		t.Fatalf("Profile mismatch after atomic replace: got %q want %q", got.Profile, "two")
	}
}

func TestSaveThenLoad_RoundTripsBootstrapFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ten", "ten.state.json")

	want := state.State{
		DotfilesRoot:     "/home/taro/dotfiles",
		Profile:          "work",
		ManagedResources: map[string]state.Resource{},
	}

	if err := state.Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := state.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.DotfilesRoot != want.DotfilesRoot {
		t.Fatalf("DotfilesRoot mismatch: got %q want %q", got.DotfilesRoot, want.DotfilesRoot)
	}
	if got.Profile != want.Profile {
		t.Fatalf("Profile mismatch: got %q want %q", got.Profile, want.Profile)
	}
}
