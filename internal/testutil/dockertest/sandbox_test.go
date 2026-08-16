package dockertest_test

import (
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/testutil/dockertest"
)

func TestSandbox_RunsTenHelp(t *testing.T) {
	sb := dockertest.NewSandbox(t)

	out, code := sb.Run(t, sb.Home(), "--help")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (output: %s)", code, out)
	}
	if !strings.Contains(out, "ten is an idempotent dotfiles manager") {
		t.Fatalf("expected help output, got: %s", out)
	}
}

func TestSandbox_WriteAndReadFile(t *testing.T) {
	sb := dockertest.NewSandbox(t)

	sb.WriteFile(t, sb.Home()+"/greeting.txt", "hello sandbox\n")
	got := sb.ReadFile(t, sb.Home()+"/greeting.txt")
	if got != "hello sandbox\n" {
		t.Fatalf("got %q", got)
	}
}
