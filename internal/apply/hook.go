package apply

import (
	"fmt"
	"io"
	"os/exec"
)

// RunHook executes cmdStr through a shell (`sh -c`) with dir as its
// working directory, streaming both stdout and stderr to out so the user
// sees what the hook is doing. Pinning the cwd (callers pass the
// dotfiles root) keeps hooks reproducible: without it a hook's behavior
// would depend on where the user happened to invoke `ten apply`. An
// empty dir falls back to the inherited cwd.
//
// It is a no-op (returns nil immediately) when cmdStr is empty. A
// non-zero exit status is returned as an error so the caller can fail
// fast, exactly like a link or template failure.
func RunHook(cmdStr, dir string, out io.Writer) error {
	if cmdStr == "" {
		return nil
	}
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apply: hook %q: %w", cmdStr, err)
	}
	return nil
}
