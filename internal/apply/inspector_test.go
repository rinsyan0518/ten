package apply_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rinsyan0518/ten/internal/apply"
)

func TestOSInspector_InspectAndReadFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("content"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(file, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	ins := apply.NewOSInspector()

	got, err := ins.Inspect(file)
	if err != nil {
		t.Fatalf("Inspect file: %v", err)
	}
	if !got.Exists || got.IsSymlink {
		t.Fatalf("regular file misread: %+v", got)
	}

	got, err = ins.Inspect(link)
	if err != nil {
		t.Fatalf("Inspect link: %v", err)
	}
	if !got.Exists || !got.IsSymlink || got.LinkDest != file {
		t.Fatalf("symlink misread: %+v", got)
	}

	got, err = ins.Inspect(filepath.Join(dir, "missing"))
	if err != nil {
		t.Fatalf("Inspect missing: %v", err)
	}
	if got.Exists {
		t.Fatalf("missing path reported as existing: %+v", got)
	}
}
