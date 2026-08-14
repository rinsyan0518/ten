package apply

import (
	"fmt"
	"io"
	"os/exec"
)

// RunHook executes cmdStr through a shell (`sh -c`), streaming both stdout
// and stderr to out so the user sees what the hook is doing.
//
// It is a no-op (returns nil immediately) when cmdStr is empty or when
// dryRun is true — under --dry-run the hook is only reported in the plan,
// never executed. A non-zero exit status is returned as an error so the
// caller can fail fast, exactly like a link or template failure.
func RunHook(cmdStr string, out io.Writer, dryRun bool) error {
	if cmdStr == "" || dryRun {
		return nil
	}
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apply: hook %q: %w", cmdStr, err)
	}
	return nil
}
