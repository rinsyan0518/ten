package apply

import (
	"fmt"
	"os"
	"runtime"
)

// SystemInfo is exposed to templates as `.Ten`, alongside `.Vars`. Every
// field except Tool is constant for one apply run; Tool is set by the
// caller to the owning tool's name immediately before that tool's
// templates are rendered.
//
// Every field must stay a plain string (no slice/map/pointer): Apply
// builds one SystemInfo per run and hands out per-tool copies by value
// (see the template case in run.go) — a reference-typed field would
// silently become shared mutable state across tools.
type SystemInfo struct {
	OS           string
	Arch         string
	Hostname     string
	Home         string
	Profile      string
	Tool         string
	DotfilesRoot string
}

// NewSystemInfo resolves the machine-dependent field (Hostname) once
// per run. Home/Profile/DotfilesRoot are passed straight through since
// the caller already has them. Tool is left zero-valued — callers set
// it per tool before rendering that tool's templates.
func NewSystemInfo(home, profile, dotfilesRoot string) (SystemInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return SystemInfo{}, fmt.Errorf("apply: resolve hostname: %w", err)
	}
	return SystemInfo{
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		Hostname:     hostname,
		Home:         home,
		Profile:      profile,
		DotfilesRoot: dotfilesRoot,
	}, nil
}
