// Package tencli wraps dockertest.Sandbox with helpers for driving the
// ten binary itself, keeping dockertest free of any knowledge of ten's
// CLI.
package tencli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/testutil/dockertest"
)

// Sandbox wraps a dockertest.Sandbox with helpers for running the ten
// binary inside it.
type Sandbox struct {
	*dockertest.Sandbox
}

// NewSandbox creates a fresh sandbox for exercising the ten binary.
func NewSandbox(t *testing.T) *Sandbox {
	t.Helper()
	return &Sandbox{dockertest.NewSandbox(t)}
}

// shellQuote wraps s in single quotes for safe use as one word in a
// POSIX shell command, escaping any single quotes it contains. Plain
// space-joining args (the previous approach) silently dropped
// empty-string args and mishandled any arg containing whitespace.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Run executes the ten binary inside the sandbox with the given HOME and
// arguments.
func (s *Sandbox) Run(t *testing.T, home string, args ...string) (stdout string, exitCode int) {
	t.Helper()
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = shellQuote(arg)
	}
	out, _, code := s.Exec(t, fmt.Sprintf("HOME=%s ten %s", shellQuote(home), strings.Join(quoted, " ")))
	return out, code
}

// Init creates root and runs `ten init --path root` (optionally with
// --profile) inside the sandbox, failing the test if init exits
// non-zero.
func (s *Sandbox) Init(t *testing.T, home, root string, profile ...string) {
	t.Helper()
	s.Exec(t, "mkdir -p "+root)
	args := []string{"init", "--path", root}
	if len(profile) > 0 {
		args = append(args, "--profile", profile[0])
	}
	if _, exitCode := s.Run(t, home, args...); exitCode != 0 {
		t.Fatalf("tencli: ten init --path %s failed with exit code %d", root, exitCode)
	}
}
