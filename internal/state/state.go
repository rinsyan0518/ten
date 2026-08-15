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
// needed.
func Save(path string, s State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
