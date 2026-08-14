package main

import "runtime/debug"

// version reports a version string for --version. Users mostly install
// ten via `go install .../ten@version`, which Go stamps into the binary's
// module version automatically (bi.Main.Version); use that directly. A
// local `go build` instead gets "(devel)", so fall back to the VCS
// revision Go also stamps automatically, so builds are still traceable
// to a commit without a separate release/ldflags pipeline.
func version() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}

	var revision string
	var dirty bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if revision == "" {
		return "(devel)"
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if dirty {
		return "(devel)+" + revision + "-dirty"
	}
	return "(devel)+" + revision
}
