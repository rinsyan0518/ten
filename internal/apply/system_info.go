package apply

import "github.com/rinsyan0518/ten/internal/render"

// SystemInfo aliases render.SystemInfo, which is where the `.Ten`
// template context now lives; the alias keeps this package's
// Executor/RunParams signatures readable.
type SystemInfo = render.SystemInfo
