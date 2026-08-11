package apply

import (
	"fmt"
	"os"
	"path/filepath"
)

// LinkResult describes the outcome of applying a single symlink target.
type LinkResult struct {
	Target     string
	Source     string
	BackupPath string // empty if no backup was made
	Skipped    bool   // true if the link already existed and was correct
}

// Link ensures target does not yet exist and creates a symlink from
// target to source. If target already exists, Link returns an error (the
// backup-and-idempotency behavior is added in a later task). If dryRun is
// true, no filesystem changes are made.
func Link(target, source, backupDir string, dryRun bool) (LinkResult, error) {
	if _, err := os.Lstat(target); err == nil {
		return LinkResult{}, fmt.Errorf("apply: target already exists: %s", target)
	} else if !os.IsNotExist(err) {
		return LinkResult{}, fmt.Errorf("apply: lstat %s: %w", target, err)
	}

	if dryRun {
		return LinkResult{Target: target, Source: source}, nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return LinkResult{}, fmt.Errorf("apply: mkdir for %s: %w", target, err)
	}
	if err := os.Symlink(source, target); err != nil {
		return LinkResult{}, fmt.Errorf("apply: symlink %s -> %s: %w", target, source, err)
	}
	return LinkResult{Target: target, Source: source}, nil
}
