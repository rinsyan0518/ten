package apply

import "io"

// Executor performs the filesystem/process side effects an apply or
// destroy run needs. NewOSExecutor returns the real implementation used
// in production; tests substitute a fake so Apply/Destroy can be
// exercised without touching disk or spawning processes.
type Executor interface {
	Link(target, source, backupDir string, dryRun bool) (LinkResult, error)
	Unlink(req UnlinkRequest, dryRun bool) (UnlinkResult, error)
	RenderTemplate(target, source string, vars map[string]string, backupDir string, alreadyManaged, dryRun bool) (TemplateResult, error)
	RunHook(cmdStr string, out io.Writer, dryRun bool) error
}

type osExecutor struct{}

// NewOSExecutor returns the Executor backed by this package's real
// Link/Unlink/RenderTemplate/RunHook functions.
func NewOSExecutor() Executor { return osExecutor{} }

func (osExecutor) Link(target, source, backupDir string, dryRun bool) (LinkResult, error) {
	return Link(target, source, backupDir, dryRun)
}

func (osExecutor) Unlink(req UnlinkRequest, dryRun bool) (UnlinkResult, error) {
	return Unlink(req, dryRun)
}

func (osExecutor) RenderTemplate(target, source string, vars map[string]string, backupDir string, alreadyManaged, dryRun bool) (TemplateResult, error) {
	return RenderTemplate(target, source, vars, backupDir, alreadyManaged, dryRun)
}

func (osExecutor) RunHook(cmdStr string, out io.Writer, dryRun bool) error {
	return RunHook(cmdStr, out, dryRun)
}

var _ Executor = osExecutor{}
