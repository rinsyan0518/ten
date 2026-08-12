package config

// Tool is a single [tools.X] definition, shared by ten.toml,
// ten.<profile>.toml, and ten.local.toml.
type Tool struct {
	Links     map[string]string `toml:"links"`
	Templates map[string]string `toml:"templates"`
	DependsOn []string          `toml:"depends_on"`
	PreApply  string            `toml:"pre_apply"`
	PostApply string            `toml:"post_apply"`
}

// Core holds machine-local settings from ten.local.toml's [core] table.
type Core struct {
	DotfilesRoot string `toml:"dotfiles_root"`
	Profile      string `toml:"profile"`
}

// Local is the parsed contents of ten.local.toml.
type Local struct {
	Core  Core              `toml:"core"`
	Vars  map[string]string `toml:"vars"`
	Tools map[string]Tool   `toml:"tools"`
}

// Repo is the parsed contents of a repository config file (ten.toml or
// ten.<profile>.toml).
type Repo struct {
	Tools map[string]Tool `toml:"tools"`
}

// Merged is the fully resolved configuration used by the rest of the
// pipeline (graph, plan, apply).
type Merged struct {
	DotfilesRoot string
	Vars         map[string]string
	Tools        map[string]Tool
	Enabled      map[string]bool
}
