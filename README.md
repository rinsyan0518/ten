# ten (点)

[![CI](https://github.com/rinsyan0518/ten/actions/workflows/ci.yml/badge.svg)](https://github.com/rinsyan0518/ten/actions/workflows/ci.yml)

> **⚠️ `ten` is under development. Breaking changes may land without notice.**

English | [日本語](README.ja.md)

`ten` is a Go CLI dotfiles manager. Its simple, declarative design doesn't lock you into a particular way of working, so you can move to another tool anytime you need to.

- **Idempotent** — running it repeatedly always converges to the same result
- **Safe backups** — existing files are backed up before anything is overwritten, under `$XDG_STATE_HOME/ten/backup` next to ten's state (never as a dotfile in `$HOME`)
- **Dependency resolution** — tools apply in DAG order via `depends_on`
- **Per-machine differences** — profile-specific files plus local overrides
- **Stateful garbage collection** — resources removed from config are automatically cleaned up
- **No imposed layout** — `links`/`templates` source paths are just paths in your repo; `ten` doesn't require any particular directory convention
- **Destroy** — `ten destroy` tears down everything it manages and restores backups

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/rinsyan0518/ten/main/install.sh | sh
```

Linux and macOS (amd64/arm64) only. For `go install`, building from source, and install options, see [Installation](docs/guide/installation.md).

## Example

```toml
# ~/dotfiles/ten.toml
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }

[tools.nvim]
links = { "xdg:nvim" = "nvim" }
after = "nvim --headless '+Lazy! sync' +qa"
```

```bash
cd ~/dotfiles
ten init              # point ten at this repo
ten apply --dry-run   # preview what would change
ten apply             # apply for real
```

## Documentation

**Guide**

- [Installation](docs/guide/installation.md)
- [Quick start](docs/guide/quick-start.md)

**Concepts**

- [How apply works](docs/concepts/apply.md) — apply flow, dependency order, hook execution

**Reference**

- [Configuration](docs/reference/configuration.md) — target path prefixes, `[tools.*]`, template data, config layering
- [Commands](docs/reference/commands.md)

**Development**

- [Development](docs/development/development.md) — building, testing, `make` targets

## License

[MIT](LICENSE)
