.PHONY: build test lint fmt ci

build:
	go build -o ten ./cmd/ten

test:
	go test -p 1 ./...

lint:
	golangci-lint run

fmt:
	gofmt -l -w .

ci: build lint test
