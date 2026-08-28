package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Resource is a single filesystem resource ten created and now manages.
type Resource struct {
	Tool       string `json:"tool"`
	Type       string `json:"type"` // "symlink" | "template"
	Source     string `json:"source"`
	BackupPath string `json:"backup_path,omitempty"`
}

// State is the parsed contents of ten.state.json.
type State struct {
	DotfilesRoot     string              `json:"dotfiles_root,omitempty"`
	Profile          string              `json:"profile,omitempty"`
	LastApplied      time.Time           `json:"last_applied"`
	ManagedResources map[string]Resource `json:"managed_resources"`
}

// Load reads and parses the state file at path. If the file does not
// exist, it returns an empty State and no error (first run).
func Load(path string) (State, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return State{ManagedResources: map[string]Resource{}}, nil
	}
	if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, err
	}
	if s.ManagedResources == nil {
		s.ManagedResources = map[string]Resource{}
	}
	return s, nil
}

// Save writes s to path as indented JSON, creating parent directories as
// needed. The file is written to a temporary sibling and renamed into
// place, never truncated in place: ten.state.json is the only record of
// what ten manages, and a crash mid-write must leave the previous state
// intact rather than a half-written file that blocks every later
// apply/destroy/init.
func Save(path string, s State) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() {
		// No-ops on success (the file has been renamed away by then);
		// on any failure they discard the partial temp file.
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	if _, err := tmp.Write(data); err != nil {
		return err
	}
	// Flush to disk before the rename makes the new file visible, so a
	// crash cannot promote an unwritten file over the previous state.
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
