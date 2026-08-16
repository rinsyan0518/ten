package config

// Tool is a single [tools.X] definition, shared by ten.toml,
// ten.<profile>.toml, and ten.local.toml.
type Tool struct {
	Enabled   *bool             `toml:"enabled"`
	Links     map[string]string `toml:"links"`
	Templates map[string]string `toml:"templates"`
	DependsOn []string          `toml:"depends_on"`
	PreApply  string            `toml:"pre_apply"`
	PostApply string            `toml:"post_apply"`
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
