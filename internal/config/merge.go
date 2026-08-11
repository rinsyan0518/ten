package config

// Merge combines a repo config and local config into a single Merged
// configuration. Tool definitions are combined with whole-tool replace: a
// tool defined in both repo and local is fully replaced by local's
// definition (no field-level merging).
func Merge(repo Repo, local Local) Merged {
	tools := make(map[string]Tool, len(repo.Tools)+len(local.Tools))
	for name, t := range repo.Tools {
		tools[name] = t
	}
	for name, t := range local.Tools {
		tools[name] = t
	}

	return Merged{
		DotfilesRoot: local.Core.DotfilesRoot,
		Vars:         local.Vars,
		Tools:        tools,
	}
}
