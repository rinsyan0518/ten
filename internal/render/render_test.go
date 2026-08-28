package render_test

import (
	"runtime"
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/render"
)

func TestRender_ExposesVarsAndTenContext(t *testing.T) {
	body := "email={{ .Vars.git_email }} os={{ .Ten.OS }} tool={{ .Ten.Tool }}\n"
	ten := render.SystemInfo{OS: "testos", Tool: "git"}

	got, err := render.Render("test.tmpl", []byte(body), map[string]string{"git_email": "taro@example.com"}, ten)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "email=taro@example.com os=testos tool=git\n"
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRender_UndefinedVarIsAnError(t *testing.T) {
	_, err := render.Render("test.tmpl", []byte("{{ .Vars.missing }}"), map[string]string{}, render.SystemInfo{})
	if err == nil {
		t.Fatalf("expected error for undefined .Vars.missing, got nil")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("error should name the missing key, got: %v", err)
	}
}

func TestNewSystemInfo_ResolvesRuntimeAndPassthroughFields(t *testing.T) {
	got, err := render.NewSystemInfo("/home/taro", "work", "/dotfiles")
	if err != nil {
		t.Fatalf("NewSystemInfo: %v", err)
	}
	if got.OS != runtime.GOOS {
		t.Fatalf("OS = %q, want %q", got.OS, runtime.GOOS)
	}
	if got.Arch != runtime.GOARCH {
		t.Fatalf("Arch = %q, want %q", got.Arch, runtime.GOARCH)
	}
	if got.Hostname == "" {
		t.Fatal("Hostname is empty")
	}
	if got.Home != "/home/taro" {
		t.Fatalf("Home = %q, want %q", got.Home, "/home/taro")
	}
	if got.Profile != "work" {
		t.Fatalf("Profile = %q, want %q", got.Profile, "work")
	}
	if got.DotfilesRoot != "/dotfiles" {
		t.Fatalf("DotfilesRoot = %q, want %q", got.DotfilesRoot, "/dotfiles")
	}
	if got.Tool != "" {
		t.Fatalf("Tool should be zero-valued by NewSystemInfo, got %q", got.Tool)
	}
}
