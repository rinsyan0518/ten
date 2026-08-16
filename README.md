# ten (点)

[![CI](https://github.com/rinsyan0518/ten/actions/workflows/ci.yml/badge.svg)](https://github.com/rinsyan0518/ten/actions/workflows/ci.yml)

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

| File | Location | Track in your dotfiles repo? | Role |
|---|---|---|---|
| `ten.toml` / `ten.<profile>.toml` | `<dotfiles_root>/` | Yes | Desired state — which tools go where |
| `ten.local.toml` | `<dotfiles_root>/ten.local.toml` | No (add to `.gitignore`) | Machine-local settings: secret vars and local tool overrides |
| `ten.state.json` | `$XDG_STATE_HOME/ten/ten.state.json` (falls back to `~/.local/state/ten/ten.state.json`) | N/A (lives outside the repo) | Bootstrap pointer (`dotfiles_root`/`profile`, set by `ten init`) plus an auto-generated record of what `ten` currently manages |

### 1. Point ten at your dotfiles repo

```bash
cd ~/dotfiles
ten init                                    # dotfiles_root defaults to the current directory
# or: ten init --path ~/dotfiles --profile work
```

### 2. Add `ten.toml` to your dotfiles repo

```toml
# ~/dotfiles/ten.toml
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }

[tools.nvim]
links = { "xdg:nvim" = "nvim" }
post_apply = "nvim --headless '+Lazy! sync' +qa"
```

### 3. (Optional) Add machine-local overrides

`ten.local.toml` lives inside your dotfiles repo alongside `ten.toml`, so add it to `.gitignore` before creating it:

```bash
echo ten.local.toml >> ~/dotfiles/.gitignore
```

```toml
# ~/dotfiles/ten.local.toml — gitignored, not committed
[vars]
git_email = "taro.yamada@work.example.com"
git_name = "Taro Yamada"
```

### 4. Apply

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
enabled    = false                                                   # defaults to true when omitted
depends_on = ["git"]                                                 # applied after its dependencies, in DAG order
links     = { "home:.gitconfig" = "git/.gitconfig" }                 # symlinks
templates = { "home:.gitconfig.local" = "git/gitconfig.local.tmpl" } # rendered via text/template ({{ .Vars.key }})
pre_apply  = "echo before"                                           # shell command run before this tool's resources
post_apply = "echo after"                                            # shell command run after
```

`[tools.*]` definitions are layered `ten.toml` → `ten.<profile>.toml` → `ten.local.toml`, merged **per field**: a later layer overrides only the fields it actually sets, leaving fields it leaves unset untouched from the earlier layer. `links` / `templates` / `depends_on` are each replaced as a whole when a later layer sets them at all (no per-key merging within a single field). `pre_apply` / `post_apply` treat an empty string as "not set," so a later layer can't explicitly clear a value set by an earlier layer.

Use `enabled` to turn a tool on or off per layer without repeating its other fields — e.g. define a tool disabled by default in `ten.toml` and flip it on for one profile:

```toml
# ten.toml (common, always loaded)
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }

[tools.git-work]
enabled    = false
depends_on = ["git"]
templates  = { "home:.gitconfig.local" = "git/gitconfig.local.tmpl" }

# ten.work.toml (loaded only when profile = "work")
[tools.git-work]
enabled = true
```

`vars` follows the same base → profile → local layering but merges per variable key rather than per tool: a variable declared in a later layer overrides only that one key, leaving variables declared solely in earlier layers untouched.

## Commands

```
ten init [--path <path>] [--profile <name>]   Point ten at a dotfiles repository (--path defaults to the current directory)
ten apply [--dry-run]                         Apply every tool resolved by the current profile, in DAG order
ten destroy [--dry-run]                       Remove everything ten manages, in reverse order (restoring backups where they exist)
```

Neither `ten apply` nor `ten destroy` supports targeting individual tools — what gets applied or destroyed is controlled declaratively via each tool's `enabled` field.

`ten init`'s `--profile` leaves the existing profile unchanged when omitted; pass `--profile ""` explicitly to clear it.

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
