# 開発

[English](development.md) | 日本語

一部のテスト（`cmd/ten`）はビルド済みの `ten` バイナリを実際のファイルシステムに書き込みながらエンドツーエンドで動かし、hookコマンドも実行するため、ローカル環境を汚さないようDockerサンドボックス内でのみ実行されます。実行にはDockerが必要です。

```bash
make build      # go build -o bin/ten ./cmd/ten
make test       # go test -p 1 ./...
make lint       # golangci-lint run
make fmt        # golangci-lint fmt (rewrites files)
make fmt-check  # golangci-lint fmt --diff (fails if formatting is needed)
make vulncheck  # govulncheck ./...
make ci         # build + fmt-check + lint + vulncheck + test
```
