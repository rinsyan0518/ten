# ten (点)

[![CI](https://github.com/rinsyan0518/ten/actions/workflows/ci.yml/badge.svg)](https://github.com/rinsyan0518/ten/actions/workflows/ci.yml)

> **⚠️ `ten` はまだ開発段階です。予告なく破壊的変更が入ることがあります。**

[English](README.md) | 日本語

`ten` はGo製のdotfilesマネージャCLIです。シンプルな仕組みで宣言的にdotfilesを管理でき、特定の使い方に縛られないため、必要になったら他のツールへいつでも移行できます。

- **冪等性** — 何度実行しても同じ結果に収束する
- **安全なバックアップ** — 上書きする前に既存ファイルをバックアップする。保存先は state と同じ `$XDG_STATE_HOME/ten/backup` で、`$HOME` 直下にドットファイルを増やさない
- **依存関係解決** — ツールは `depends_on` によるDAG順序で適用される
- **マシンごとの差分** — プロファイル別ファイル＋ローカルオーバーライド
- **ステートフルなガベージコレクション** — 設定から削除されたリソースは自動的にクリーンアップされる
- **ディレクトリ構造を強制しない** — `links`/`templates` の参照元パスはリポジトリ内の単なるパスであり、`ten` は特定のディレクトリ構成を要求しない
- **Destroy** — `ten destroy` は管理しているものすべてを取り壊し、バックアップを復元する

## インストール

```bash
curl -fsSL https://raw.githubusercontent.com/rinsyan0518/ten/main/install.sh | sh
```

LinuxとmacOS（amd64/arm64）のみ対応しています。`go install`、ソースからのビルド、インストールオプションは[インストール](docs/guide/installation.ja.md)を参照してください。

## 例

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
ten init              # このリポジトリをtenに指定する
ten apply --dry-run   # 変更内容をプレビュー
ten apply             # 実際に適用
```

## ドキュメント

**ガイド**

- [インストール](docs/guide/installation.ja.md)
- [クイックスタート](docs/guide/quick-start.ja.md)

**コンセプト**

- [applyの仕組み](docs/concepts/apply.ja.md) — applyの流れ、依存関係の順序、hookの実行

**リファレンス**

- [設定](docs/reference/configuration.ja.md) — ターゲットパスのプレフィックス、`[tools.*]`、テンプレートで使える値、設定のレイヤリング
- [コマンド](docs/reference/commands.ja.md)

**開発**

- [開発](docs/development/development.ja.md) — ビルド、テスト、`make` ターゲット

## ライセンス

[MIT](LICENSE)
