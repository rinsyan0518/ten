package apply

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rinsyan0518/ten/internal/pathresolve"
	"github.com/rinsyan0518/ten/internal/state"
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
	// ContentHash is the recorded hash of the template output ten last
	// wrote ("template" resources only; empty on pre-hashing records).
	ContentHash string
	// BackupRoot bounds the empty-directory cleanup after a restore
	// (typically ~/.ten_backup): directories emptied by moving the
	// backup away are removed up to and including this root, never
	// beyond it. Empty disables the cleanup.
	BackupRoot string
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
// The target is verified first (see verifyOwned): a symlink must still
// point at req.Source, and template output must still hash to
// req.ContentHash. If it doesn't, the user has replaced ten's resource
// with something of their own and Unlink refuses to touch it
// (Skipped=true) instead of silently destroying it.
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
		// Moving the backup away may have emptied its timestamp directory
		// (and mirrored path components under it); sweep those so
		// ~/.ten_backup doesn't accumulate empty skeletons forever. Best
		// effort — a failure here never fails the restore itself.
		if req.BackupRoot != "" {
			pruneEmptyDirs(filepath.Dir(req.BackupPath), req.BackupRoot)
		}
		return UnlinkResult{Target: req.Target, Restored: true}, nil
	}

	if err := os.RemoveAll(req.Target); err != nil {
		return UnlinkResult{}, fmt.Errorf("apply: remove %s: %w", req.Target, err)
	}
	return UnlinkResult{Target: req.Target}, nil
}

// pruneEmptyDirs removes dir and then each parent in turn, stopping at
// the first directory that isn't empty (os.Remove refuses non-empty
// directories) and never climbing past root. root itself is removed too
// when it ends up empty.
func pruneEmptyDirs(dir, root string) {
	root = filepath.Clean(root)
	for dir = filepath.Clean(dir); ; dir = filepath.Dir(dir) {
		rel, err := filepath.Rel(root, dir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return // outside root — never touch anything up here
		}
		if err := os.Remove(dir); err != nil {
			return // not empty (or already gone with a non-empty parent)
		}
		if dir == root {
			return
		}
	}
}

// verifyOwned returns an empty string if the existing target still looks
// like the resource ten recorded, or a human-readable reason if it does
// not. A symlink is verified by its destination; a template by the hash
// of the content ten last wrote (records from before content hashing
// carry no hash and pass unverified, as they always did).
func verifyOwned(req UnlinkRequest, info os.FileInfo) string {
	switch req.Type {
	case "symlink":
		if info.Mode()&os.ModeSymlink == 0 {
			return "no longer a symlink created by ten"
		}
		dest, err := os.Readlink(req.Target)
		if err != nil {
			return fmt.Sprintf("could not read symlink (%v)", err)
		}
		if !pathresolve.EqualPaths(dest, req.Source) {
			return fmt.Sprintf("symlink now points at %s, not %s", dest, req.Source)
		}
	case "template":
		if req.ContentHash == "" {
			return ""
		}
		if !info.Mode().IsRegular() {
			return "no longer a regular file written by ten"
		}
		content, err := os.ReadFile(req.Target)
		if err != nil {
			return fmt.Sprintf("could not read file (%v)", err)
		}
		if state.HashContent(content) != req.ContentHash {
			return "content changed since ten wrote it"
		}
	}
	return ""
}
