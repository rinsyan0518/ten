package apply_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/apply"
)

func TestRunHook_RunsInTheGivenDirectory(t *testing.T) {
	dir := t.TempDir()

	var out testWriter
	if err := apply.RunHook("pwd", dir, &out); err != nil {
		t.Fatalf("RunHook: %v", err)
	}
	// On macOS t.TempDir may be a symlinked path (/var -> /private/var);
	// compare resolved paths so the assertion checks the directory, not
	// its spelling.
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	got, err := filepath.EvalSymlinks(strings.TrimSpace(out.String()))
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", out.String(), err)
	}
	if got != want {
		t.Fatalf("hook ran in %q, want %q", got, want)
	}
}

func TestRunHook_EmptyDirFallsBackToInheritedCwd(t *testing.T) {
	var out testWriter
	if err := apply.RunHook("pwd", "", &out); err != nil {
		t.Fatalf("RunHook: %v", err)
	}
	if strings.TrimSpace(out.String()) == "" {
		t.Fatalf("expected pwd output, got %q", out.String())
	}
}
