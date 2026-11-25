# Quick start

English | [日本語](quick-start.ja.md)

`ten` works with three kinds of config/state:

| File | Location | Track in your dotfiles repo? | Role |
|---|---|---|---|
| `ten.toml` / `ten.<profile>.toml` | `<dotfiles_root>/` | Yes | Desired state — which tools go where |
| `ten.local.toml` | `<dotfiles_root>/ten.local.toml` | No (add to `.gitignore`) | Machine-local settings: secret vars and local tool overrides |
| `ten.state.json` | `$XDG_STATE_HOME/ten/ten.state.json` (falls back to `~/.local/state/ten/ten.state.json`) | N/A (lives outside the repo) | Bootstrap pointer (`dotfiles_root`/`profile`, set by `ten init`) plus an auto-generated record of what `ten` currently manages |

## 1. Point ten at your dotfiles repo

```bash
cd ~/dotfiles
ten init                                    # dotfiles_root defaults to the current directory
# or: ten init --path ~/dotfiles --profile work
```

## 2. Add `ten.toml` to your dotfiles repo

```toml
# ~/dotfiles/ten.toml
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }

[tools.nvim]
links = { "xdg:nvim" = "nvim" }
after = "nvim --headless '+Lazy! sync' +qa"
```

See the [configuration reference](../reference/configuration.md) for every field.

## 3. (Optional) Add machine-local overrides

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

These vars are consumed by `templates`, e.g.:

```
# ~/dotfiles/git/gitconfig.local.tmpl
[user]
    email = {{ .Vars.git_email }}
    name = {{ .Vars.git_name }}
```

Templates can also reference `.Ten.*` values (OS, hostname, profile, …) that `ten` resolves itself — see [Template data](../reference/configuration.md#template-data).

## 4. Apply

```bash
ten apply --dry-run   # preview what would change
ten apply             # apply for real
```

To understand what happens during apply, see [How apply works](../concepts/apply.md).

## 5. (Optional) Undo

```bash
ten destroy --dry-run   # preview what would be removed
ten destroy              # remove for real, restoring backups
```
