package apply_test

import (
	"runtime"
	"testing"

	"github.com/rinsyan0518/ten/internal/apply"
)

func TestNewSystemInfo_ResolvesRuntimeAndPassthroughFields(t *testing.T) {
	got, err := apply.NewSystemInfo("/home/taro", "work", "/dotfiles")
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
