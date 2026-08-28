package config

// Tool is a single [tools.X] definition, shared by ten.toml,
// ten.<profile>.toml, and ten.local.toml.
type Tool struct {
	Enabled   *bool             `toml:"enabled"`
	Links     map[string]string `toml:"links"`
	Templates map[string]string `toml:"templates"`
	DependsOn []string          `toml:"depends_on"`
	Before    string            `toml:"before"`
	// Once runs a shell command the first time this tool newly manages a
	// link or template — i.e. when plan.Build finds at least one of the
	// tool's desired targets absent from state.State.ManagedResources at
	// the start of the run. It never fires again once a target is
	// tracked, and never fires for a tool with no links/templates. A
	// failed once leaves those targets untracked so it re-arms next run.
	Once  string `toml:"once"`
	After string `toml:"after"`
}

// File is the parsed contents of any of ten.toml, ten.<profile>.toml, or
// ten.local.toml — all three share the same schema. What distinguishes
// them is layering order and location (ten.local.toml lives inside the
// dotfiles repo too, but is meant to be gitignored), not structure.
type File struct {
	Vars  map[string]string `toml:"vars"`
	Tools map[string]Tool   `toml:"tools"`
}

// Merged is the fully resolved configuration used by the rest of the
// pipeline (graph, plan, apply). DotfilesRoot is set by the caller of
// Merge (cmd/ten's loadMerged) from state.State.DotfilesRoot, not by
// Merge itself — File no longer carries a dotfiles_root field.
type Merged struct {
	DotfilesRoot string
	Vars         map[string]string
	Tools        map[string]Tool
	Enabled      map[string]bool
}
