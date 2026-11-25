# コマンド

[English](commands.md) | 日本語

```
ten init [--path <path>] [--profile <name>]   dotfilesリポジトリをtenに指定する（--path省略時はカレントディレクトリ）
ten apply [--dry-run]                         現在のプロファイルで解決されたすべてのツールをDAG順序で適用する
ten destroy [--dry-run]                       ten.state.jsonのみを使って（設定不要で）tenが管理しているものをすべて削除し、存在する場合はバックアップを復元する
```

`ten apply` も `ten destroy` も個別ツールを指定した実行はサポートしていません — 何が適用/削除されるかは、各ツールの `enabled` フィールドによって宣言的に制御されます。

`ten destroy`(および apply の prune)は削除の前に、各リソースが今も ten の作ったものであることを検証します: シンボリックリンクは記録された source を指し続けていること、テンプレート出力は ten が最後に書いた内容のままであること。ユーザーが置き換えたり編集したりしたものは、削除される代わりに警告付きでスキップされます。

`ten destroy` はhooksを一切実行しません — `depends_on` はapply時のhook実行順序を決めるためだけのものであり、destroyはそれを完全に無視します。また `ten.state.json` を削除することもありません: 管理しているすべてのリソースを削除・復元したあと、管理リソースが空の状態でファイルを書き直します（`ten init` が設定したブートストラップ用フィールド（`dotfiles_root`/`profile`）は保持されます）。

`ten init` の `--profile` は省略時、既存のプロファイルをそのまま変更しません。クリアしたい場合は `--profile ""` を明示的に指定してください。
