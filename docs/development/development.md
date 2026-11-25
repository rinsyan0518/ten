# Development

English | [日本語](development.ja.md)

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
