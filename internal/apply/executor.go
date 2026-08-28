package apply

import "io"

// Executor performs the filesystem/process side effects an Execute or
// ExecuteDestroy run needs. NewOSExecutor returns the real
// implementation used in production; tests substitute a fake so the
// engines can be exercised without touching disk or spawning processes.
// Dry-run never constructs an Executor at all: it stops at the plan.
type Executor interface {
	Link(target, source, backupDir string) (LinkResult, error)
	Unlink(req UnlinkRequest) (UnlinkResult, error)
	// RenderTemplate renders source with vars/ten and writes the result
	// to target. backup requests that whatever currently occupies target
	// be backed up under backupDir first; without it, a target whose
	// content already equals the render is left untouched
	// (TemplateResult.Skipped).
	RenderTemplate(target, source string, vars map[string]string, ten SystemInfo, backupDir string, backup bool) (TemplateResult, error)
	// RunHook runs cmdStr through a shell with dir as its working
	// directory (the dotfiles root).
	RunHook(cmdStr, dir string, out io.Writer) error
}

type osExecutor struct{}

// NewOSExecutor returns the Executor backed by this package's real
// Link/Unlink/RenderTemplate/RunHook functions.
func NewOSExecutor() Executor { return osExecutor{} }

func (osExecutor) Link(target, source, backupDir string) (LinkResult, error) {
	return Link(target, source, backupDir)
}

func (osExecutor) Unlink(req UnlinkRequest) (UnlinkResult, error) {
	return Unlink(req)
}

func (osExecutor) RenderTemplate(target, source string, vars map[string]string, ten SystemInfo, backupDir string, backup bool) (TemplateResult, error) {
	return RenderTemplate(target, source, vars, ten, backupDir, backup)
}

func (osExecutor) RunHook(cmdStr, dir string, out io.Writer) error {
	return RunHook(cmdStr, dir, out)
}

var _ Executor = osExecutor{}
