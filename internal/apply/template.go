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
// resolves as described in the spec.
type templateContext struct {
	Vars map[string]string
}

// RenderTemplate renders the template at sourcePath using vars as
// context and writes the result to target.
//
// If alreadyManaged is false and target already exists, the existing
// file is backed up under backupDir first (same scheme as Link). If
// alreadyManaged is true, target is overwritten in place with no backup
// (idempotent re-render — used once state tracking exists).
func RenderTemplate(target, sourcePath string, vars map[string]string, backupDir string, alreadyManaged, dryRun bool) (TemplateResult, error) {
	tmplBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return TemplateResult{}, fmt.Errorf("apply: read template %s: %w", sourcePath, err)
	}
	tmpl, err := template.New(filepath.Base(sourcePath)).Parse(string(tmplBytes))
	if err != nil {
		return TemplateResult{}, fmt.Errorf("apply: parse template %s: %w", sourcePath, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateContext{Vars: vars}); err != nil {
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
	if err := os.WriteFile(target, buf.Bytes(), 0o644); err != nil {
		return TemplateResult{}, fmt.Errorf("apply: write template output %s: %w", target, err)
	}
	return TemplateResult{Target: target, Source: sourcePath, BackupPath: backupPath}, nil
}
