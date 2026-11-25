# Commands

English | [日本語](commands.ja.md)

```
ten init [--path <path>] [--profile <name>]   Point ten at a dotfiles repository (--path defaults to the current directory)
ten apply [--dry-run]                         Apply every tool resolved by the current profile, in DAG order
ten destroy [--dry-run]                       Remove everything ten manages using ten.state.json only (no config required), restoring backups where they exist
```

Neither `ten apply` nor `ten destroy` supports targeting individual tools — what gets applied or destroyed is controlled declaratively via each tool's `enabled` field.

Before removing anything, `ten destroy` (and apply's pruning) verifies each resource is still the one ten created: a symlink must still point at its recorded source, and template output must still match the content ten last wrote. Anything you've since replaced or edited is skipped with a warning instead of deleted.

`ten destroy` never runs hooks — `depends_on` only orders hook execution during `apply`, and destroy ignores it entirely. It also doesn't delete `ten.state.json`: after removing or restoring every managed resource, it rewrites the file with an empty managed-resources record, keeping the bootstrap fields (`dotfiles_root`/`profile`) set by `ten init`.

`ten init`'s `--profile` leaves the existing profile unchanged when omitted; pass `--profile ""` explicitly to clear it.
