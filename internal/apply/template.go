package apply

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rinsyan0518/ten/internal/render"
	"github.com/rinsyan0518/ten/internal/state"
)

// TemplateResult describes the outcome of rendering a single template.
type TemplateResult struct {
	Target     string
	Source     string
	BackupPath string
	// Skipped is set when the target already held exactly the rendered
	// content and nothing was written (idempotent re-apply).
	Skipped bool
	// ContentHash is state.HashContent of the rendered output, reported
	// on the skip path too so pre-hashing state records get backfilled
	// by an idempotent re-apply.
	ContentHash string
}

// RenderTemplate renders the template at source using vars and ten as
// context and writes the result to target. It runs at execution time —
// after the owning tool's before hook — because a hook may generate or
// update the source.
//
// If backup is true (the plan saw a foreign file at target), whatever
// occupies target is backed up under backupDir before writing. Without
// backup, the target is ten's own previous output: when its content
// already equals the fresh render, nothing is written and the result is
// marked Skipped, so an idempotent re-apply neither rewrites the file
// nor churns its mtime.
func RenderTemplate(target, source string, vars map[string]string, ten SystemInfo, backupDir string, backup bool) (TemplateResult, error) {
	text, err := os.ReadFile(source)
	if err != nil {
		return TemplateResult{}, fmt.Errorf("apply: read template %s: %w", source, err)
	}
	content, err := render.Render(filepath.Base(source), text, vars, ten)
	if err != nil {
		return TemplateResult{}, fmt.Errorf("apply: %w", err)
	}
	contentHash := state.HashContent(content)

	info, lstatErr := os.Lstat(target)
	exists := lstatErr == nil
	if lstatErr != nil && !os.IsNotExist(lstatErr) {
		return TemplateResult{}, fmt.Errorf("apply: lstat %s: %w", target, lstatErr)
	}
	// Template output typically carries machine-local values from vars —
	// often secrets — so it is always owner-only: group/other permission
	// bits are never carried over. A brand-new file gets 0600; a regular
	// file being replaced or updated keeps only its owner bits (so an
	// owner-executable script stays executable), which also tightens a
	// world-readable file, including output an older ten wrote as 0644.
	mode := os.FileMode(0o600)
	if exists && info.Mode().IsRegular() {
		mode = info.Mode().Perm() &^ 0o077
	}

	var backupPath string
	switch {
	case backup && exists:
		backupPath = filepath.Join(backupDir, time.Now().Format("20060102_150405"), stripLeadingSlash(target))
		if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
			return TemplateResult{}, fmt.Errorf("apply: mkdir backup dir for %s: %w", target, err)
		}
		if err := os.Rename(target, backupPath); err != nil {
			return TemplateResult{}, fmt.Errorf("apply: backup %s: %w", target, err)
		}
	case !backup && exists && info.Mode().IsRegular():
		// Overwriting ten's own previous output: compare first so an
		// unchanged template is reported up to date instead of rewritten.
		current, err := os.ReadFile(target)
		if err != nil {
			return TemplateResult{}, fmt.Errorf("apply: read %s: %w", target, err)
		}
		if bytes.Equal(current, content) {
			// Up to date content still gets its mode tightened, or output
			// written world-readable by an older ten would stay that way
			// for as long as its content never changes.
			if info.Mode().Perm() != mode {
				if err := os.Chmod(target, mode); err != nil {
					return TemplateResult{}, fmt.Errorf("apply: chmod %s: %w", target, err)
				}
			}
			return TemplateResult{Target: target, Source: source, Skipped: true, ContentHash: contentHash}, nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return TemplateResult{}, fmt.Errorf("apply: mkdir for %s: %w", target, err)
	}
	if err := writeFileAtomic(target, content, mode); err != nil {
		return TemplateResult{}, fmt.Errorf("apply: write template output %s: %w", target, err)
	}
	return TemplateResult{Target: target, Source: source, BackupPath: backupPath, ContentHash: contentHash}, nil
}

// writeFileAtomic writes content to a temporary sibling of target with the
// given mode and renames it into place. A crash mid-write therefore never
// leaves a truncated config file behind, and because rename(2) replaces
// the directory entry itself, a symlink sitting at target is replaced
// rather than written through — so a stale symlink can never make ten
// corrupt whatever it points at (typically a file in the dotfiles repo).
func writeFileAtomic(target string, content []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".ten-tmp-*")
	if err != nil {
		return err
	}
	defer func() {
		// No-ops on success (the file has been renamed away by then); on
		// any failure they discard the partial temp file.
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	if _, err := tmp.Write(content); err != nil {
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), target)
}
