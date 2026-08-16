package e2e_test

import (
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/testutil/tencli"
)

func TestInit_SandboxInitWithEmptyProfileClearsProfile(t *testing.T) {
	sb := tencli.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles", "work")
	stateJSON := sb.ReadFile(t, home+"/.local/state/ten/ten.state.json")
	if !strings.Contains(stateJSON, `"profile": "work"`) {
		t.Fatalf("expected profile to be set to work, got: %s", stateJSON)
	}

	// An empty profile arg must round-trip through Sandbox.Run's shell
	// quoting intact rather than being dropped, or --profile would be
	// left without a value and cobra would fail to parse it.
	sb.Init(t, home, home+"/dotfiles", "")
	stateJSON = sb.ReadFile(t, home+"/.local/state/ten/ten.state.json")
	if strings.Contains(stateJSON, `"profile"`) {
		t.Fatalf("expected profile to be cleared, got: %s", stateJSON)
	}
}
