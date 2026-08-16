package dockertest

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
)

// sharedContainer is built and started lazily, on the first NewSandbox
// call in a test binary, and torn down by RunWithSharedContainer after
// all tests finish. It's shared across every NewSandbox call in that
// binary rather than rebuilt per test, since building and stopping a
// container is the dominant cost of each test. Starting it lazily (rather
// than unconditionally in RunWithSharedContainer) means test files in the
// same package that never touch a sandbox stay fast and Docker-free.
var (
	sharedContainer testcontainers.Container
	sharedOnce      sync.Once
	sharedErr       error
)

// RunWithSharedContainer runs m and then, if any test called NewSandbox,
// terminates the shared container it started. Call it from a package's
// TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(dockertest.RunWithSharedContainer(m)) }
func RunWithSharedContainer(m *testing.M) int {
	code := m.Run()

	if sharedContainer != nil {
		if err := sharedContainer.Terminate(context.Background(), testcontainers.StopTimeout(0)); err != nil {
			fmt.Fprintf(os.Stderr, "dockertest: terminate shared sandbox container: %v\n", err)
		}
	}
	return code
}

func startSharedContainer() (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			// Every caller is a package two directories under the repo root
			// (test/e2e, internal/apply, internal/dockertest), and Go test
			// binaries run with their package directory as the working
			// directory.
			Context:    "../..",
			Dockerfile: "Dockerfile.test",
		},
		Cmd: []string{"sleep", "infinity"},
	}
	return testcontainers.GenericContainer(context.Background(), testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

// Sandbox is an isolated home directory inside the shared sandbox
// container, used to exercise apply/destroy without touching the host
// filesystem.
type Sandbox struct {
	container testcontainers.Container
	home      string
}

// NewSandbox creates a fresh, uniquely named home directory for t inside
// the container started by RunWithSharedContainer.
func NewSandbox(t *testing.T) *Sandbox {
	t.Helper()
	sharedOnce.Do(func() {
		sharedContainer, sharedErr = startSharedContainer()
	})
	if sharedErr != nil {
		t.Fatalf("dockertest: start shared sandbox container: %v", sharedErr)
	}

	home := "/root/" + sanitizeTestName(t.Name())
	sb := &Sandbox{container: sharedContainer, home: home}
	if _, _, code := sb.Exec(t, "mkdir -p "+home); code != 0 {
		t.Fatalf("dockertest: create home dir %s failed (exit %d)", home, code)
	}
	return sb
}

// sanitizeTestName maps a test name to a safe single path segment; Go test
// names can contain "/" (subtests) and spaces (table-driven names).
func sanitizeTestName(name string) string {
	return strings.NewReplacer("/", "_", " ", "_").Replace(name)
}

// Home returns the sandbox's HOME directory path (inside the container).
func (s *Sandbox) Home() string {
	return s.home
}

// Exec runs an arbitrary shell command inside the sandbox and returns its
// combined output and exit code.
func (s *Sandbox) Exec(t *testing.T, shellCmd string) (stdout, stderr string, exitCode int) {
	t.Helper()
	code, reader, err := s.container.Exec(context.Background(), []string{"sh", "-c", shellCmd}, tcexec.Multiplexed())
	if err != nil {
		t.Fatalf("dockertest: exec %q: %v", shellCmd, err)
	}
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, reader); err != nil {
		t.Fatalf("dockertest: read exec output: %v", err)
	}
	return buf.String(), "", code
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
		t.Fatalf("dockertest: ten init --path %s failed with exit code %d", root, exitCode)
	}
}

// WriteFile writes content to path inside the sandbox, creating parent
// directories as needed.
func (s *Sandbox) WriteFile(t *testing.T, path, content string) {
	t.Helper()
	dir := path[:strings.LastIndex(path, "/")]
	cmd := fmt.Sprintf("mkdir -p %s && cat > %s <<'TENEOF'\n%sTENEOF\n", dir, path, content)
	if _, _, code := s.Exec(t, cmd); code != 0 {
		t.Fatalf("dockertest: write file %s failed (exit %d)", path, code)
	}
}

// ReadFile reads the content of path inside the sandbox.
func (s *Sandbox) ReadFile(t *testing.T, path string) string {
	t.Helper()
	out, _, code := s.Exec(t, fmt.Sprintf("cat %s", path))
	if code != 0 {
		t.Fatalf("dockertest: read file %s failed (exit %d)", path, code)
	}
	return out
}

// Lstat reports whether path exists inside the sandbox and, if it's a
// symlink, what it points to. ok is false if the path does not exist.
func (s *Sandbox) Lstat(t *testing.T, path string) (isSymlink bool, target string, ok bool) {
	t.Helper()
	if _, _, code := s.Exec(t, fmt.Sprintf("test -e %s -o -L %s", path, path)); code != 0 {
		return false, "", false
	}
	out, _, code := s.Exec(t, fmt.Sprintf("readlink %s", path))
	if code == 0 && strings.TrimSpace(out) != "" {
		return true, strings.TrimSpace(out), true
	}
	return false, "", true
}
