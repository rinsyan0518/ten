package apply

import (
	"fmt"
	"os"
	"path/filepath"
)

// UnlinkResult describes the outcome of removing or restoring a single
// managed resource.
type UnlinkResult struct {
	Target   string
	Restored bool
}

// Unlink removes the resource at target. If backupPath is non-empty, the
// backup is restored to target instead of being deleted.
func Unlink(target, backupPath string, dryRun bool) (UnlinkResult, error) {
	if dryRun {
		return UnlinkResult{Target: target, Restored: backupPath != ""}, nil
	}
	if backupPath != "" {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return UnlinkResult{}, fmt.Errorf("apply: mkdir for restore %s: %w", target, err)
		}
		if err := os.Rename(backupPath, target); err != nil {
			return UnlinkResult{}, fmt.Errorf("apply: restore backup %s -> %s: %w", backupPath, target, err)
		}
		return UnlinkResult{Target: target, Restored: true}, nil
	}
	if err := os.RemoveAll(target); err != nil {
		return UnlinkResult{}, fmt.Errorf("apply: remove %s: %w", target, err)
	}
	return UnlinkResult{Target: target}, nil
}
