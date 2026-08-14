GOLANGCI_LINT_VERSION := v2.12.2
GOLANGCI_LINT := bin/golangci-lint

.PHONY: build test lint fmt fmt-check ci

build:
	go build -o bin/ten ./cmd/ten

test:
	go test -p 1 ./...

lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

fmt: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt

fmt-check: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt --diff

ci: build fmt-check lint test

# Installed into a repo-local ./bin (not GOPATH/bin) so this doesn't
# overwrite a golangci-lint version another project on the machine
# depends on. Built in isolation via `go install <pkg>@version`, which
# does not touch this module's go.mod/go.sum — golangci-lint vendors
# ~100 linters, and pulling that in as a real dependency (e.g. via
# `go get -tool`) would bloat go.sum with unrelated transitive deps.
$(GOLANGCI_LINT):
	GOBIN=$(CURDIR)/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
