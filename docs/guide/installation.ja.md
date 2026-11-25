# インストール

[English](installation.md) | 日本語

ビルド済みバイナリはLinuxとmacOS（amd64/arm64）向けに配布されています。Windowsには対応していません。

とりあえず試したいだけなら、インストールスクリプトを実行してください。最新リリースをダウンロードし、チェックサムを検証したうえで `$HOME/.local/bin`（`INSTALL_DIR` で変更可能）にインストールします。バージョンを固定したい場合は `TEN_VERSION` を指定してください:

```bash
curl -fsSL https://raw.githubusercontent.com/rinsyan0518/ten/main/install.sh | sh
```

またはGo経由で:

```bash
go install github.com/rinsyan0518/ten/cmd/ten@latest
```

またはクローンしてビルド:

```bash
git clone https://github.com/rinsyan0518/ten.git
cd ten
go build -o ten ./cmd/ten
```

次へ: [クイックスタート](quick-start.ja.md)
