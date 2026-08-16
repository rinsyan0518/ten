package e2e_test

import (
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/testutil/dockertest"
)

func TestCLI_RejectsUnexpectedPositionalArguments(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/.gitconfig", "base\n")

	// Targeted apply/destroy does not exist, so an extra argument is a
	// typo and must not silently run against everything.
	out, code := sb.Run(t, home, "apply", "somearg")
	if code == 0 {
		t.Fatalf("expected `ten apply somearg` to fail, got exit 0: %s", out)
	}
	if !strings.Contains(out, "somearg") {
		t.Fatalf("expected the error to name the unexpected argument, got: %s", out)
	}
	if _, _, ok := sb.Lstat(t, home+"/.gitconfig"); ok {
		t.Fatalf("expected `ten apply somearg` not to apply anything")
	}

	if out, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("apply failed (exit %d): %s", code, out)
	}

	out, code = sb.Run(t, home, "destroy", "somearg")
	if code == 0 {
		t.Fatalf("expected `ten destroy somearg` to fail, got exit 0: %s", out)
	}
	if !strings.Contains(out, "somearg") {
		t.Fatalf("expected the error to name the unexpected argument, got: %s", out)
	}
	if isLink, _, ok := sb.Lstat(t, home+"/.gitconfig"); !ok || !isLink {
		t.Fatalf("expected `ten destroy somearg` not to destroy anything")
	}
}
