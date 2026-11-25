# クイックスタート

[English](quick-start.md) | 日本語

`ten` は3種類の設定/状態ファイルを扱います:

| ファイル | 配置場所 | dotfilesリポジトリで管理する? | 役割 |
|---|---|---|---|
| `ten.toml` / `ten.<profile>.toml` | `<dotfiles_root>/` | する | 望ましい状態 — どのツールをどこに配置するか |
| `ten.local.toml` | `<dotfiles_root>/ten.local.toml` | しない（`.gitignore` に追加） | マシンローカルな設定: 秘密の変数やツールのローカルオーバーライド |
| `ten.state.json` | `$XDG_STATE_HOME/ten/ten.state.json`（フォールバック先: `~/.local/state/ten/ten.state.json`） | 対象外（リポジトリの外に存在） | ブートストラップ用のポインタ（`ten init` が設定する `dotfiles_root`/`profile`）と、`ten` が現在管理しているものの自動生成された記録 |

## 1. tenにdotfilesリポジトリを指定する

```bash
cd ~/dotfiles
ten init                                    # dotfiles_root はデフォルトでカレントディレクトリになる
# または: ten init --path ~/dotfiles --profile work
```

## 2. dotfilesリポジトリに `ten.toml` を追加する

```toml
# ~/dotfiles/ten.toml
[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig" }

[tools.nvim]
links = { "xdg:nvim" = "nvim" }
after = "nvim --headless '+Lazy! sync' +qa"
```

すべてのフィールドは[設定リファレンス](../reference/configuration.ja.md)を参照してください。

## 3. （任意）マシンローカルなオーバーライドを追加する

`ten.local.toml` は `ten.toml` と同じくdotfilesリポジトリ内に置くため、作成する前に `.gitignore` に追加してください:

```bash
echo ten.local.toml >> ~/dotfiles/.gitignore
```

```toml
# ~/dotfiles/ten.local.toml — gitignore対象、コミットしない
[vars]
git_email = "taro.yamada@work.example.com"
git_name = "Taro Yamada"
```

これらの変数は `templates` から次のように参照されます:

```
# ~/dotfiles/git/gitconfig.local.tmpl
[user]
    email = {{ .Vars.git_email }}
    name = {{ .Vars.git_name }}
```

テンプレートからは、`ten` が自分で解決する `.Ten.*` の値（OS、ホスト名、プロファイルなど）も参照できます — [テンプレートで使える値](../reference/configuration.ja.md#テンプレートで使える値)を参照してください。

## 4. 適用する

```bash
ten apply --dry-run   # 変更内容をプレビュー
ten apply             # 実際に適用
```

apply で何が起きるかは[applyの仕組み](../concepts/apply.ja.md)を参照してください。

## 5. （任意）取り消す

```bash
ten destroy --dry-run   # 削除内容をプレビュー
ten destroy              # 実際に削除し、バックアップを復元
```
