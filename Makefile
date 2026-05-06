GO        ?= go
BINARY    ?= goove
PKG       := ./cmd/goove

GOLANGCI_LINT_VERSION ?= v1.64.8
GOVULNCHECK_VERSION   ?= latest

.DEFAULT_GOAL := help

.PHONY: help build run install test test-race test-integration vet fmt fmt-check lint vuln tools ci clean

help:  ## list targets
	@awk 'BEGIN{FS=":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build:  ## build the goove binary
	$(GO) build -o $(BINARY) $(PKG)

run:  ## run from source
	$(GO) run $(PKG)

install:  ## go install into $GOBIN
	$(GO) install $(PKG)

test:  ## unit tests
	$(GO) test ./...

test-race:  ## unit tests with the race detector
	$(GO) test -race ./...

test-integration:  ## integration tests (hits real Music.app)
	$(GO) test -tags=integration ./internal/music/applescript/

vet:  ## go vet
	$(GO) vet ./...

fmt:  ## format the tree (writes changes)
	gofmt -w .

fmt-check:  ## fail if any file is unformatted
	@diff=$$(gofmt -l .); if [ -n "$$diff" ]; then echo "gofmt needed:"; echo "$$diff"; exit 1; fi

lint:  ## golangci-lint
	golangci-lint run

vuln:  ## govulncheck
	govulncheck ./...

tools:  ## install pinned dev tools into $GOBIN
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	$(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

ci: fmt-check vet lint vuln test-race build  ## what CI runs

clean:  ## remove built binaries
	rm -f $(BINARY) main
