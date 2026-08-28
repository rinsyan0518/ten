// Package render is the single home of ten's template rendering: the
// `.Vars`/`.Ten` context exposed to templates and the missingkey policy.
// Both the planning phase (rendering in memory to compare against what
// is on disk) and the execution phase (writing the rendered bytes) go
// through Render, so the two can never disagree about what a template
// produces.
package render

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"text/template"
)

// SystemInfo is exposed to templates as `.Ten`, alongside `.Vars`. Every
// field except Tool is constant for one apply run; Tool is set by the
// caller to the owning tool's name immediately before that tool's
// templates are rendered.
//
// Every field must stay a plain string (no slice/map/pointer): callers
// hand out per-tool copies by value — a reference-typed field would
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
		return SystemInfo{}, fmt.Errorf("render: resolve hostname: %w", err)
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

// context is exposed to templates as ".", so `{{ .Vars.key }}` and
// `{{ .Ten.key }}` resolve as described in the spec.
type context struct {
	Vars map[string]string
	Ten  SystemInfo
}

// Render renders the template text with vars and ten as context. name is
// used in error messages only (typically the source file's base name).
//
// missingkey=error: a reference to an undefined .Vars key must fail the
// render, not silently produce "<no value>" in the output (e.g. a
// gitconfig on a machine whose ten.local.toml is missing or unreadable).
func Render(name string, text []byte, vars map[string]string, ten SystemInfo) ([]byte, error) {
	tmpl, err := template.New(name).Option("missingkey=error").Parse(string(text))
	if err != nil {
		return nil, fmt.Errorf("render: parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, context{Vars: vars, Ten: ten}); err != nil {
		return nil, fmt.Errorf("render: render template %s: %w", name, err)
	}
	return buf.Bytes(), nil
}
