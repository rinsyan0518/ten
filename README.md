# ten (点)

`ten` is a Go CLI dotfiles manager.

- **Idempotent** — running it repeatedly always converges to the same result
- **Safe backups** — existing files are backed up before anything is overwritten
- **Dependency resolution** — tools apply in DAG order via `depends_on`
- **Per-machine differences** — profile-specific files plus local overrides
- **Stateful garbage collection** — resources removed from config are automatically cleaned up
- **Destroy** — `ten destroy` tears down everything it manages and restores backups

## Install

```bash
go install github.com/rinsyan0518/ten/cmd/ten@latest
```

Or clone and build:

```bash
git clone https://github.com/rinsyan0518/ten.git
cd ten
go build -o ten ./cmd/ten
```

## Quick start

`ten` works with three kinds of config/state:

| File | Location | Git-tracked | Role |
|---|---|---|---|
| `ten.local.toml` | `~/.config/ten/ten.local.toml` | No | Machine-local settings (`dotfiles_root`, `profile`, secret vars, etc.) |
| `ten.toml` / `ten.<profile>.toml` | `<dotfiles_root>/` | Yes | Desired state — which tools go where |
| `ten.state.json` | `~/.config/ten/ten.state.json` | No | Auto-generated record of what `ten` currently manages |

### 1. Create your machine-local config

```toml
# ~/.config/ten/ten.local.toml
[core]
dotfiles_root = "~/dotfiles"
profile = "work" # optional; controls which ten.<profile>.toml gets loaded

[vars]
git_email = "taro.yamada@work.example.com"
git_name = "Taro Yamada"
```

### 2. Add `ten.toml` to your dotfiles repo

```toml
# ~/dotfiles/ten.toml
enabled_tools = ["git", "nvim"]

[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }

[tools.nvim]
links = { "xdg:nvim" = "nvim" }
post_apply = "nvim --headless '+Lazy! sync' +qa"
```

### 3. Apply

```bash
ten apply --dry-run   # preview what would change
ten apply             # apply for real
```

## Configuration reference

### Target path prefixes

Keys in `links` / `templates` resolve to absolute paths using one of these prefixes:

- `home:` → under `$HOME`
- `xdg:` → under `$XDG_CONFIG_HOME` (falls back to `$HOME/.config`)
- `custom:` → any absolute path (`~/` expansion supported)

### `[tools.*]`

```toml
[tools.git-work]
depends_on = ["git"]                                                 # applied after its dependencies, in DAG order
links     = { "home:.gitconfig" = "git/.gitconfig" }                 # symlinks
templates = { "home:.gitconfig.local" = "git/gitconfig.local.tmpl" } # rendered via text/template ({{ .Vars.key }})
pre_apply  = "echo before"                                           # shell command run before this tool's resources
post_apply = "echo after"                                            # shell command run after
```

### `enabled_tools`

Can be declared in `ten.toml`, `ten.<profile>.toml`, and/or `ten.local.toml` — the final enabled set is the union across all of them. If none of them declare it, every defined tool is enabled by default (backward-compatible fallback).

```toml
# ten.toml (common, always loaded)
enabled_tools = ["git", "nvim"]

# ten.work.toml (loaded only when profile = "work")
enabled_tools = ["git-work"]
```

`[tools.*]` definitions themselves are layered `ten.toml` → `ten.<profile>.toml` → `ten.local.toml`, with each layer fully replacing a same-named tool (whole-tool replace, no field-level merging).

## Commands

```
ten apply [--dry-run]     Apply every tool resolved by the current profile, in DAG order
ten destroy [--dry-run]   Remove everything ten manages, in reverse order (restoring backups where they exist)
```

Neither command supports targeting individual tools — what gets applied or destroyed is controlled declaratively via `enabled_tools`.

## Development

Some tests (`cmd/ten`) drive the built `ten` binary end to end, writing to a real filesystem and executing hook commands, so they run exclusively inside a Docker sandbox to avoid touching your local environment. Docker is required to run them.

```bash
make build      # go build -o bin/ten ./cmd/ten
make test       # go test -p 1 ./...
make lint       # golangci-lint run
make fmt        # golangci-lint fmt (rewrites files)
make fmt-check  # golangci-lint fmt --diff (fails if formatting is needed)
make vulncheck  # govulncheck ./...
make ci         # build + fmt-check + lint + vulncheck + test
```

## License

[MIT](LICENSE)
