package apply_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/state"
)

func writeSource(t *testing.T, dir, content string) string {
	t.Helper()
	src := filepath.Join(dir, "source.tmpl")
	if err := os.WriteFile(src, []byte(content), 0o644); err != nil {
		t.Fatalf("write source template: %v", err)
	}
	return src
}

func TestRenderTemplate_RendersVarsAndTenCreatingParentDirs(t *testing.T) {
	dir := t.TempDir()
	src := writeSource(t, dir, "email={{ .Vars.git_email }} tool={{ .Ten.Tool }}\n")
	target := filepath.Join(dir, "nested", "target.txt")

	result, err := apply.RenderTemplate(target, src, map[string]string{"git_email": "taro@example.com"}, apply.SystemInfo{Tool: "git"}, filepath.Join(dir, "backup"), false)
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "email=taro@example.com tool=git\n" {
		t.Fatalf("got %q", got)
	}
	if result.BackupPath != "" || result.Skipped {
		t.Fatalf("fresh render should have no backup and not be skipped: %+v", result)
	}
	if result.ContentHash != state.HashContent(got) {
		t.Fatalf("result should carry the rendered content's hash, got %q", result.ContentHash)
	}
}

func TestRenderTemplate_UndefinedVarIsAnErrorAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	src := writeSource(t, dir, "email = {{ .Vars.git_email }}\n")
	target := filepath.Join(dir, "target.txt")

	_, err := apply.RenderTemplate(target, src, map[string]string{}, apply.SystemInfo{}, dir, false)
	if err == nil {
		t.Fatalf("expected error for undefined .Vars.git_email, got nil")
	}
	if !strings.Contains(err.Error(), "git_email") {
		t.Fatalf("error should name the missing key, got: %v", err)
	}
	if _, statErr := os.Lstat(target); !os.IsNotExist(statErr) {
		t.Fatalf("target must not be written when rendering fails, lstat: %v", statErr)
	}
}

func TestRenderTemplate_BacksUpForeignFileWhenRequested(t *testing.T) {
	dir := t.TempDir()
	src := writeSource(t, dir, "rendered")
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("user's own"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	result, err := apply.RenderTemplate(target, src, nil, apply.SystemInfo{}, filepath.Join(dir, "backup"), true)
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if result.BackupPath == "" {
		t.Fatalf("expected a backup path")
	}
	backed, err := os.ReadFile(result.BackupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backed) != "user's own" {
		t.Fatalf("backup content = %q", backed)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "rendered" {
		t.Fatalf("target = %q", got)
	}
}

func TestRenderTemplate_UnchangedOutputIsSkippedWithoutRewrite(t *testing.T) {
	dir := t.TempDir()
	src := writeSource(t, dir, "same content")
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("same content"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	before, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	result, err := apply.RenderTemplate(target, src, nil, apply.SystemInfo{}, filepath.Join(dir, "backup"), false)
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if !result.Skipped {
		t.Fatalf("expected Skipped for identical content, got %+v", result)
	}
	// The hash must be reported even on the skip path, so state entries
	// written before content hashing existed get backfilled by an
	// idempotent re-apply without rewriting the file.
	if result.ContentHash != state.HashContent([]byte("same content")) {
		t.Fatalf("skip path should still report the content hash, got %q", result.ContentHash)
	}
	after, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("an up-to-date template must not be rewritten (mtime churn)")
	}
}

func TestRenderTemplate_StaleOutputIsRewritten(t *testing.T) {
	dir := t.TempDir()
	src := writeSource(t, dir, "new content")
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("old content"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	result, err := apply.RenderTemplate(target, src, nil, apply.SystemInfo{}, filepath.Join(dir, "backup"), false)
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if result.Skipped || result.BackupPath != "" {
		t.Fatalf("stale managed output should be rewritten in place without backup: %+v", result)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new content" {
		t.Fatalf("target = %q", got)
	}
}

func TestRenderTemplate_NeverWritesThroughASymlink(t *testing.T) {
	dir := t.TempDir()
	src := writeSource(t, dir, "rendered")
	repoFile := filepath.Join(dir, "repo-file")
	if err := os.WriteFile(repoFile, []byte("precious repo content"), 0o644); err != nil {
		t.Fatalf("seed repo file: %v", err)
	}
	target := filepath.Join(dir, "target.txt")
	if err := os.Symlink(repoFile, target); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// backup=false models the update-in-place path where state says the
	// target is ten's own template output but disk disagrees.
	if _, err := apply.RenderTemplate(target, src, nil, apply.SystemInfo{}, filepath.Join(dir, "backup"), false); err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	repo, _ := os.ReadFile(repoFile)
	if string(repo) != "precious repo content" {
		t.Fatalf("symlink destination was corrupted: %q", repo)
	}
	info, err := os.Lstat(target)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("target should now be a regular file, info=%v err=%v", info, err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "rendered" {
		t.Fatalf("target = %q", got)
	}
}

func TestRenderTemplate_NewOutputIsOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	src := writeSource(t, dir, "token={{ .Vars.token }}\n")
	target := filepath.Join(dir, ".npmrc")

	if _, err := apply.RenderTemplate(target, src, map[string]string{"token": "s3cret"}, apply.SystemInfo{}, filepath.Join(dir, "backup"), false); err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("new template output mode = %v, want 0600", got)
	}
}

func TestRenderTemplate_ReplacingAFileKeepsOnlyItsOwnerBits(t *testing.T) {
	tests := []struct{ mode, want os.FileMode }{
		{0o600, 0o600},
		{0o640, 0o600},
		{0o644, 0o600},
		{0o700, 0o700},
		{0o755, 0o700},
	}
	for _, tt := range tests {
		mode := tt.mode
		t.Run(mode.String(), func(t *testing.T) {
			dir := t.TempDir()
			src := writeSource(t, dir, "machine x password {{ .Vars.pw }}\n")
			target := filepath.Join(dir, ".netrc")
			if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
				t.Fatalf("seed target: %v", err)
			}
			if err := os.Chmod(target, mode); err != nil {
				t.Fatalf("chmod target: %v", err)
			}

			if _, err := apply.RenderTemplate(target, src, map[string]string{"pw": "s3cret"}, apply.SystemInfo{}, filepath.Join(dir, "backup"), true); err != nil {
				t.Fatalf("RenderTemplate: %v", err)
			}
			info, err := os.Stat(target)
			if err != nil {
				t.Fatalf("stat target: %v", err)
			}
			if got := info.Mode().Perm(); got != tt.want {
				t.Fatalf("mode after replace of a %v file = %v, want %v", mode, got, tt.want)
			}
		})
	}
}

func TestRenderTemplate_UpdateDropsGroupOtherBitsAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	src := writeSource(t, dir, "v={{ .Vars.v }}\n")
	target := filepath.Join(dir, "out", "conf")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("v=old\n"), 0o640); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	if _, err := apply.RenderTemplate(target, src, map[string]string{"v": "new"}, apply.SystemInfo{}, filepath.Join(dir, "backup"), false); err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if got, _ := os.ReadFile(target); string(got) != "v=new\n" {
		t.Fatalf("got %q", got)
	}
	info, _ := os.Stat(target)
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode after update of a 0640 file = %v, want 0600", got)
	}
	entries, _ := os.ReadDir(filepath.Dir(target))
	if len(entries) != 1 {
		t.Fatalf("expected only the target in its directory, found %v", entries)
	}
}

func TestRenderTemplate_StaleSymlinkAtTargetIsReplacedNotWrittenThrough(t *testing.T) {
	dir := t.TempDir()
	src := writeSource(t, dir, "rendered\n")
	repoFile := filepath.Join(dir, "repo-file")
	if err := os.WriteFile(repoFile, []byte("REPO\n"), 0o644); err != nil {
		t.Fatalf("seed repo file: %v", err)
	}
	target := filepath.Join(dir, "target")
	if err := os.Symlink(repoFile, target); err != nil {
		t.Fatalf("seed symlink: %v", err)
	}

	if _, err := apply.RenderTemplate(target, src, nil, apply.SystemInfo{}, filepath.Join(dir, "backup"), false); err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if got, _ := os.ReadFile(repoFile); string(got) != "REPO\n" {
		t.Fatalf("the symlink's destination must not be written through, got %q", got)
	}
	info, _ := os.Lstat(target)
	if !info.Mode().IsRegular() {
		t.Fatalf("expected target to become a regular file, mode %v", info.Mode())
	}
}

func TestRenderTemplate_UpToDateOutputStillGetsItsModeTightened(t *testing.T) {
	// Output an older ten wrote world-readable must not stay that way just
	// because its content never changes again.
	dir := t.TempDir()
	src := writeSource(t, dir, "token={{ .Vars.token }}\n")
	target := filepath.Join(dir, ".npmrc")
	if err := os.WriteFile(target, []byte("token=s3cret\n"), 0o600); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatalf("chmod target: %v", err)
	}

	result, err := apply.RenderTemplate(target, src, map[string]string{"token": "s3cret"}, apply.SystemInfo{}, filepath.Join(dir, "backup"), false)
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if !result.Skipped {
		t.Fatalf("expected unchanged content to be reported up to date, got %+v", result)
	}
	info, _ := os.Stat(target)
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode of up-to-date output = %v, want 0600", got)
	}
}
