package apply

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"
)

// TemplateResult describes the outcome of rendering a single template.
type TemplateResult struct {
	Target     string
	Source     string
	BackupPath string
}

// templateContext is exposed to templates as ".", so `{{ .Vars.key }}`
// and `{{ .Ten.key }}` resolve as described in the spec.
type templateContext struct {
	Vars map[string]string
	Ten  SystemInfo
}

// RenderTemplate renders the template at sourcePath using vars and ten
// as context and writes the result to target.
//
// If alreadyManaged is false and target already exists, the existing
// file is backed up under backupDir first (same scheme as Link). If
// alreadyManaged is true, target is overwritten in place with no backup
// (idempotent re-render — used once state tracking exists).
func RenderTemplate(target, sourcePath string, vars map[string]string, ten SystemInfo, backupDir string, alreadyManaged, dryRun bool) (TemplateResult, error) {
	tmplBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return TemplateResult{}, fmt.Errorf("apply: read template %s: %w", sourcePath, err)
	}
	// missingkey=error: a reference to an undefined .Vars key must fail
	// the render, not silently write "<no value>" into the target (e.g. a
	// gitconfig on a machine whose ten.local.toml is missing or unreadable).
	tmpl, err := template.New(filepath.Base(sourcePath)).Option("missingkey=error").Parse(string(tmplBytes))
	if err != nil {
		return TemplateResult{}, fmt.Errorf("apply: parse template %s: %w", sourcePath, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateContext{Vars: vars, Ten: ten}); err != nil {
		return TemplateResult{}, fmt.Errorf("apply: render template %s: %w", sourcePath, err)
	}

	var backupPath string
	if !alreadyManaged {
		if _, err := os.Lstat(target); err == nil {
			backupPath = filepath.Join(backupDir, time.Now().Format("20060102_150405"), stripLeadingSlash(target))
			if !dryRun {
				if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
					return TemplateResult{}, fmt.Errorf("apply: mkdir backup dir for %s: %w", target, err)
				}
				if err := os.Rename(target, backupPath); err != nil {
					return TemplateResult{}, fmt.Errorf("apply: backup %s: %w", target, err)
				}
			}
		} else if !os.IsNotExist(err) {
			return TemplateResult{}, fmt.Errorf("apply: lstat %s: %w", target, err)
		}
	}

	if dryRun {
		return TemplateResult{Target: target, Source: sourcePath, BackupPath: backupPath}, nil
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
	if err := os.WriteFile(target, buf.Bytes(), 0o644); err != nil {
		return TemplateResult{}, fmt.Errorf("apply: write template output %s: %w", target, err)
	}
	return TemplateResult{Target: target, Source: sourcePath, BackupPath: backupPath}, nil
}
