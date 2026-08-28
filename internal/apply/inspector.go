package apply

import (
	"fmt"
	"os"

	"github.com/rinsyan0518/ten/internal/plan"
)

type osInspector struct{}

// NewOSInspector returns the real, read-only plan.Inspector backed by
// Lstat/Readlink/ReadFile. It never writes.
func NewOSInspector() plan.Inspector { return osInspector{} }

func (osInspector) Inspect(path string) (plan.Entry, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return plan.Entry{}, nil
	}
	if err != nil {
		return plan.Entry{}, fmt.Errorf("apply: lstat %s: %w", path, err)
	}
	entry := plan.Entry{Exists: true}
	if info.Mode()&os.ModeSymlink != 0 {
		entry.IsSymlink = true
		dest, err := os.Readlink(path)
		if err != nil {
			return plan.Entry{}, fmt.Errorf("apply: readlink %s: %w", path, err)
		}
		entry.LinkDest = dest
	}
	return entry, nil
}

func (osInspector) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
