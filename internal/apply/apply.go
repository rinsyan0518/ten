package apply

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LinkResult describes the outcome of applying a single symlink target.
type LinkResult struct {
	Target     string
	Source     string
	BackupPath string
	Skipped    bool
}

// Link ensures target is a symlink pointing at source. If target already
// exists and is not already that exact symlink, the existing file/dir is
// backed up under backupDir first. If target is already the correct
// symlink, Link is a no-op (Skipped=true).
func Link(target, source, backupDir string) (LinkResult, error) {
	// Refuse before creating a symlink into nothing: os.Symlink happily
	// produces a dangling link, which would then be reported as a success.
	if _, err := os.Lstat(source); err != nil {
		if os.IsNotExist(err) {
			return LinkResult{}, fmt.Errorf("apply: link source %s does not exist", source)
		}
		return LinkResult{}, fmt.Errorf("apply: lstat link source %s: %w", source, err)
	}

	info, err := os.Lstat(target)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		current, readErr := os.Readlink(target)
		if readErr != nil {
			return LinkResult{}, fmt.Errorf("apply: readlink %s: %w", target, readErr)
		}
		if current == source {
			return LinkResult{Target: target, Source: source, Skipped: true}, nil
		}
		return backupThenLink(target, source, backupDir)
	case err == nil:
		return backupThenLink(target, source, backupDir)
	case os.IsNotExist(err):
		return createLink(target, source)
	default:
		return LinkResult{}, fmt.Errorf("apply: lstat %s: %w", target, err)
	}
}

func createLink(target, source string) (LinkResult, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return LinkResult{}, fmt.Errorf("apply: mkdir for %s: %w", target, err)
	}
	if err := os.Symlink(source, target); err != nil {
		return LinkResult{}, fmt.Errorf("apply: symlink %s -> %s: %w", target, source, err)
	}
	return LinkResult{Target: target, Source: source}, nil
}

func backupThenLink(target, source, backupDir string) (LinkResult, error) {
	backupPath := filepath.Join(backupDir, time.Now().Format("20060102_150405"), stripLeadingSlash(target))
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return LinkResult{}, fmt.Errorf("apply: mkdir backup dir for %s: %w", target, err)
	}
	if err := os.Rename(target, backupPath); err != nil {
		return LinkResult{}, fmt.Errorf("apply: backup %s: %w", target, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return LinkResult{}, fmt.Errorf("apply: mkdir for %s: %w", target, err)
	}
	if err := os.Symlink(source, target); err != nil {
		return LinkResult{}, fmt.Errorf("apply: symlink %s -> %s: %w", target, source, err)
	}
	return LinkResult{Target: target, Source: source, BackupPath: backupPath}, nil
}

func stripLeadingSlash(p string) string {
	clean := filepath.Clean(p)
	if len(clean) > 0 && clean[0] == '/' {
		return clean[1:]
	}
	return clean
}
