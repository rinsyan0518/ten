package apply_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rinsyan0518/ten/internal/apply"
)

func TestRenderTemplate_ExposesTenContext(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.tmpl")
	body := "os={{ .Ten.OS }} arch={{ .Ten.Arch }} hostname={{ .Ten.Hostname }} home={{ .Ten.Home }} profile={{ .Ten.Profile }} tool={{ .Ten.Tool }} root={{ .Ten.DotfilesRoot }}\n"
	if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
		t.Fatalf("write source template: %v", err)
	}
	target := filepath.Join(dir, "target.txt")

	ten := apply.SystemInfo{
		OS:           "testos",
		Arch:         "testarch",
		Hostname:     "test-host",
		Home:         "/home/test",
		Profile:      "work",
		Tool:         "git",
		DotfilesRoot: "/dotfiles",
	}

	if _, err := apply.RenderTemplate(target, src, map[string]string{}, ten, dir, false, false); err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read rendered target: %v", err)
	}
	want := "os=testos arch=testarch hostname=test-host home=/home/test profile=work tool=git root=/dotfiles\n"
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
