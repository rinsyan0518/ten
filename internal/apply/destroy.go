package apply

import (
	"fmt"
	"os"
	"path/filepath"
)

// UnlinkRequest describes a single managed resource to take back out of
// ten's control. Type and Source mirror the ten.state.json record and are
// used to confirm the target is still the resource ten created before
// anything is deleted.
type UnlinkRequest struct {
	Target     string
	Type       string // "symlink" | "template"
	Source     string
	BackupPath string
}

// UnlinkResult describes the outcome of removing or restoring a single
// managed resource.
type UnlinkResult struct {
	Target   string
	Restored bool
	// Skipped is set when the target is no longer the resource ten
	// recorded (e.g. the user replaced ten's symlink with a real file);
	// nothing is touched and SkipReason explains why.
	Skipped    bool
	SkipReason string
}

// Unlink takes the resource at req.Target back out of ten's control. If
// req.BackupPath is non-empty the backup is restored over the target,
// otherwise the target is deleted.
//
// For "symlink" resources the target is verified first: it must still be
// a symlink pointing at req.Source. If it isn't, the user has replaced
// ten's resource with something of their own and Unlink refuses to touch
// it (Skipped=true) instead of silently destroying it. Template output is
// a plain file with no equivalent marker, so it cannot be verified this
// way and is removed as recorded.
func Unlink(req UnlinkRequest) (UnlinkResult, error) {
	info, err := os.Lstat(req.Target)
	switch {
	case err == nil:
		if reason := verifyOwned(req, info); reason != "" {
			return UnlinkResult{Target: req.Target, Skipped: true, SkipReason: reason}, nil
		}
	case !os.IsNotExist(err):
		return UnlinkResult{}, fmt.Errorf("apply: lstat %s: %w", req.Target, err)
	}
	exists := err == nil

	if req.BackupPath != "" {
		// Check the backup is really there before removing the target:
		// os.Rename cannot put back what was never there, and losing the
		// target to a restore that then fails would be unrecoverable.
		if _, statErr := os.Lstat(req.BackupPath); statErr != nil {
			return UnlinkResult{}, fmt.Errorf("apply: restore backup %s -> %s: %w", req.BackupPath, req.Target, statErr)
		}
		// os.Rename refuses to replace an existing non-directory target
		// (and a non-empty directory), so ten's own resource has to go
		// first — this is what made restoring a directory backup fail
		// with ENOTDIR.
		if exists {
			if err := os.RemoveAll(req.Target); err != nil {
				return UnlinkResult{}, fmt.Errorf("apply: remove %s before restore: %w", req.Target, err)
			}
		}
		if err := os.MkdirAll(filepath.Dir(req.Target), 0o755); err != nil {
			return UnlinkResult{}, fmt.Errorf("apply: mkdir for restore %s: %w", req.Target, err)
		}
		if err := os.Rename(req.BackupPath, req.Target); err != nil {
			return UnlinkResult{}, fmt.Errorf("apply: restore backup %s -> %s: %w", req.BackupPath, req.Target, err)
		}
		return UnlinkResult{Target: req.Target, Restored: true}, nil
	}

	if err := os.RemoveAll(req.Target); err != nil {
		return UnlinkResult{}, fmt.Errorf("apply: remove %s: %w", req.Target, err)
	}
	return UnlinkResult{Target: req.Target}, nil
}

// verifyOwned returns an empty string if the existing target still looks
// like the resource ten recorded, or a human-readable reason if it does
// not.
func verifyOwned(req UnlinkRequest, info os.FileInfo) string {
	if req.Type != "symlink" {
		return ""
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "no longer a symlink created by ten"
	}
	dest, err := os.Readlink(req.Target)
	if err != nil {
		return fmt.Sprintf("could not read symlink (%v)", err)
	}
	if dest != req.Source {
		return fmt.Sprintf("symlink now points at %s, not %s", dest, req.Source)
	}
	return ""
}
