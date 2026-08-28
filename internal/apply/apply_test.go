package apply_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rinsyan0518/ten/internal/apply"
	"github.com/rinsyan0518/ten/internal/state"
)

func TestUnlink_TemplateWithChangedContentIsSkipped(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".gitconfig.local")
	if err := os.WriteFile(target, []byte("edited by the user"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	result, err := apply.Unlink(apply.UnlinkRequest{
		Target:      target,
		Type:        "template",
		Source:      "/dotfiles/git/tmpl",
		ContentHash: state.HashContent([]byte("what ten wrote")),
	})
	if err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	if !result.Skipped {
		t.Fatalf("expected a template edited by the user to be skipped, got %+v", result)
	}
	if _, statErr := os.Lstat(target); statErr != nil {
		t.Fatalf("the edited file must be left in place: %v", statErr)
	}
}

func TestUnlink_TemplateWithMatchingContentIsRemoved(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".gitconfig.local")
	content := []byte("what ten wrote")
	if err := os.WriteFile(target, content, 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	result, err := apply.Unlink(apply.UnlinkRequest{
		Target:      target,
		Type:        "template",
		Source:      "/dotfiles/git/tmpl",
		ContentHash: state.HashContent(content),
	})
	if err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	if result.Skipped {
		t.Fatalf("expected ten's own unmodified output to be removed, got %+v", result)
	}
	if _, statErr := os.Lstat(target); !os.IsNotExist(statErr) {
		t.Fatalf("target should be gone, lstat: %v", statErr)
	}
}

func TestUnlink_RestoreCleansUpEmptiedBackupDirs(t *testing.T) {
	dir := t.TempDir()
	backupRoot := filepath.Join(dir, ".ten_backup")
	backupPath := filepath.Join(backupRoot, "20260829_120000", "home", "taro", ".gitconfig")
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(backupPath, []byte("original"), 0o644); err != nil {
		t.Fatalf("seed backup: %v", err)
	}
	target := filepath.Join(dir, ".gitconfig")

	result, err := apply.Unlink(apply.UnlinkRequest{
		Target:     target,
		Type:       "symlink",
		Source:     "/dotfiles/git/.gitconfig",
		BackupPath: backupPath,
		BackupRoot: backupRoot,
	})
	if err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	if !result.Restored {
		t.Fatalf("expected a restore, got %+v", result)
	}
	if got, _ := os.ReadFile(target); string(got) != "original" {
		t.Fatalf("restored content = %q", got)
	}
	// The restore emptied 20260829_120000/home/taro all the way up; the
	// whole now-empty tree under (and including) the backup root must be
	// gone instead of accumulating forever.
	if _, statErr := os.Lstat(backupRoot); !os.IsNotExist(statErr) {
		t.Fatalf("expected the emptied backup root to be removed, lstat: %v", statErr)
	}
}

func TestUnlink_RestoreKeepsBackupDirsThatStillHoldOtherBackups(t *testing.T) {
	dir := t.TempDir()
	backupRoot := filepath.Join(dir, ".ten_backup")
	backupPath := filepath.Join(backupRoot, "20260829_120000", "home", "taro", ".gitconfig")
	otherBackup := filepath.Join(backupRoot, "20260829_120000", "home", "taro", ".vimrc")
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for p, c := range map[string]string{backupPath: "original", otherBackup: "other"} {
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatalf("seed %s: %v", p, err)
		}
	}
	target := filepath.Join(dir, ".gitconfig")

	if _, err := apply.Unlink(apply.UnlinkRequest{
		Target:     target,
		Type:       "symlink",
		Source:     "/dotfiles/git/.gitconfig",
		BackupPath: backupPath,
		BackupRoot: backupRoot,
	}); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	if got, err := os.ReadFile(otherBackup); err != nil || string(got) != "other" {
		t.Fatalf("the sibling backup must survive: %q, %v", got, err)
	}
}

func TestLink_SymlinkDifferingOnlyInPathSpellingIsUpToDate(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "dotfiles", "git", ".gitconfig")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(source, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	target := filepath.Join(dir, ".gitconfig")
	// The existing symlink stores an uncleaned spelling of the same path
	// (e.g. written by an older ten or by hand).
	unclean := filepath.Join(dir, "dotfiles", "git") + "/../git/.gitconfig"
	if err := os.Symlink(unclean, target); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	result, err := apply.Link(target, source, filepath.Join(dir, "backup"))
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if !result.Skipped {
		t.Fatalf("expected the equivalent symlink to be left untouched, got %+v", result)
	}
	if result.BackupPath != "" {
		t.Fatalf("no backup should be taken for an equivalent symlink, got %q", result.BackupPath)
	}
}
