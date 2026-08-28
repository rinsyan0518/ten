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
