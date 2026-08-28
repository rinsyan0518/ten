package apply

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rinsyan0518/ten/internal/render"
)

// TemplateResult describes the outcome of rendering a single template.
type TemplateResult struct {
	Target     string
	Source     string
	BackupPath string
	// Skipped is set when the target already held exactly the rendered
	// content and nothing was written (idempotent re-apply).
	Skipped bool
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

	info, lstatErr := os.Lstat(target)
	exists := lstatErr == nil
	if lstatErr != nil && !os.IsNotExist(lstatErr) {
		return TemplateResult{}, fmt.Errorf("apply: lstat %s: %w", target, lstatErr)
	}
	isSymlink := exists && info.Mode()&os.ModeSymlink != 0

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
	case !backup && exists && !isSymlink:
		// Overwriting ten's own previous output: compare first so an
		// unchanged template is reported up to date instead of rewritten.
		current, err := os.ReadFile(target)
		if err != nil {
			return TemplateResult{}, fmt.Errorf("apply: read %s: %w", target, err)
		}
		if bytes.Equal(current, content) {
			return TemplateResult{Target: target, Source: source, Skipped: true}, nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return TemplateResult{}, fmt.Errorf("apply: mkdir for %s: %w", target, err)
	}
	// Never write through a symlink: os.WriteFile follows it and would
	// corrupt whatever it points at (typically a file in the user's
	// dotfiles repo). Belt and braces in case state and disk disagree.
	if info, err := os.Lstat(target); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(target); err != nil {
			return TemplateResult{}, fmt.Errorf("apply: remove symlink at template target %s: %w", target, err)
		}
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		return TemplateResult{}, fmt.Errorf("apply: write template output %s: %w", target, err)
	}
	return TemplateResult{Target: target, Source: source, BackupPath: backupPath}, nil
}
