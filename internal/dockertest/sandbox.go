package dockertest

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
)

// Sandbox is a running container with the ten binary installed, used to
// exercise apply/destroy without touching the host filesystem.
type Sandbox struct {
	container testcontainers.Container
	home      string
}

// NewSandbox builds Dockerfile.test (from the repo root) and starts a
// container from it. The container is terminated automatically when the
// test finishes.
func NewSandbox(t *testing.T) *Sandbox {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    findRepoRoot(t),
			Dockerfile: "Dockerfile.test",
		},
		Cmd: []string{"sleep", "infinity"},
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("dockertest: start sandbox container: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(context.Background()); err != nil {
			t.Logf("dockertest: terminate container: %v", err)
		}
	})

	return &Sandbox{container: c, home: "/root"}
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

// Run executes the ten binary inside the sandbox with the given HOME and
// arguments.
func (s *Sandbox) Run(t *testing.T, home string, args ...string) (stdout string, exitCode int) {
	t.Helper()
	out, _, code := s.Exec(t, fmt.Sprintf("HOME=%s ten %s", home, strings.Join(args, " ")))
	return out, code
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

func findRepoRoot(t *testing.T) string {
	t.Helper()
	// internal/dockertest -> repo root is two directories up.
	return "../.."
}
