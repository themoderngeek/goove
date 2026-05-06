# Devex tooling — Makefile, expanded CI, release artifacts

**Status:** approved
**Date:** 2026-05-06

## Goal

Improve the developer experience for goove by:

1. Adding a `Makefile` as the single source of truth for build/test/lint commands.
2. Expanding GitHub Actions CI to catch a wider class of issues on every PR.
3. Adding a release workflow that builds and publishes Mac binaries when a `v*` tag is pushed.

The headline outcome: `make ci` locally is exactly what GitHub runs on a PR, so contributors can reproduce CI failures without guessing.

## Non-goals

- No Linux/Windows support. goove is macOS-only (AppleScript bridge), CI runners stay on `macos-latest`.
- No code signing or notarisation of release binaries — users will `xattr -d` on first run.
- No multi-version Go matrix. The project pins to a single Go version (`1.24`).
- No coverage thresholds or enforcement (option E from brainstorm — declined).
- Integration tests (`-tags=integration`) are not run in CI — they require a real Music.app.

## Architecture

Three artifacts make up the contract:

| File | Role |
|---|---|
| `Makefile` (new, repo root) | Single source of truth for build/test/lint commands; humans and CI both run targets from here. |
| `.github/workflows/ci.yml` (replaces existing) | Runs on push to `main` and on PRs. Installs Go + dev tools, then runs `make ci`. |
| `.github/workflows/release.yml` (new) | Triggered by `v*` tag push. Builds `darwin-arm64` and `darwin-amd64` tarballs, attaches them to a GitHub Release. |

Two supporting files:

- `.golangci.yml` — minimal lint config.
- `.github/release-notes-footer.md` — static install instructions appended to every release.

The README's "Development" section is rewritten to reference `make` targets.

## Makefile

Located at the repo root. Default goal is `help`.

```make
GO        ?= go
BINARY    ?= goove
PKG       := ./cmd/goove

GOLANGCI_LINT_VERSION ?= v1.62.2
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
```

### Design notes

- **`tools` pins `golangci-lint` to a version, `govulncheck` to `latest`.** Linter version drift causes noisy churn; the vuln database should be as fresh as possible.
- **`ci` is the canonical chain.** `make ci` locally is exactly what GitHub runs.
- **`fmt-check` fails loud** without modifying the tree, so it's safe in CI.
- **`help` auto-generates** by scanning `##` comments — no separate help message to maintain.
- **`clean`** removes both `goove` and `main` (a stale `main` binary lives at the repo root from past builds).

## CI workflow — `.github/workflows/ci.yml`

```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:

jobs:
  ci:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          cache: true
      - name: Install dev tools
        run: make tools
      - name: Run CI
        run: make ci
```

### Design notes

- **Single job.** No matrix, no parallel split. The project is small enough that wall-clock parallelism is marginal, and one job is much easier to reason about.
- **macOS runner** is required (AppleScript bindings, Music.app SDK assumptions in tests). Non-negotiable.
- **`setup-go@v5` caches modules and the build cache** keyed on `go.sum`. This handles fast paths automatically.
- **`make tools` runs each invocation.** `go install` of `golangci-lint` is the slow step (~30s); `setup-go`'s cache absorbs most of it. If it ever feels slow, we can add an explicit `actions/cache` on `~/go/bin` — out of scope for now.
- **No `test-integration` step.** Integration tests hit Music.app which won't run on a GitHub runner. They remain available locally.

## Release workflow — `.github/workflows/release.yml`

```yaml
name: release

on:
  push:
    tags: ['v*']

permissions:
  contents: write   # required for gh release create

jobs:
  release:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          cache: true

      - name: Build darwin-arm64
        run: GOOS=darwin GOARCH=arm64 go build -o dist/goove ./cmd/goove
      - name: Package darwin-arm64
        run: tar -czf goove-${{ github.ref_name }}-darwin-arm64.tar.gz -C dist goove && rm dist/goove

      - name: Build darwin-amd64
        run: GOOS=darwin GOARCH=amd64 go build -o dist/goove ./cmd/goove
      - name: Package darwin-amd64
        run: tar -czf goove-${{ github.ref_name }}-darwin-amd64.tar.gz -C dist goove && rm dist/goove

      - name: Create GitHub Release
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          gh release create "${{ github.ref_name }}" \
            --title "${{ github.ref_name }}" \
            --generate-notes \
            --notes-file .github/release-notes-footer.md \
            goove-${{ github.ref_name }}-darwin-arm64.tar.gz \
            goove-${{ github.ref_name }}-darwin-amd64.tar.gz
```

### Design notes

- **Trigger:** tag pushes matching `v*` (e.g. `v0.1.0`). Does not fire on every push or PR.
- **Both Mac architectures** are built from a single `macos-latest` runner — `GOOS=darwin GOARCH=amd64` cross-compiles natively.
- **Tarballs** named `goove-v0.1.0-darwin-arm64.tar.gz` etc.; inside, the binary is plain `goove`.
- **`--generate-notes`** auto-writes release notes from PRs/commits since the last tag. The static install instructions are appended via `--notes-file`.
- **No `make` wrapper.** The release path is small and `GOOS`/`GOARCH`-conditional; pulling it into a `make release` target adds indirection without value.
- **Permissions:** `contents: write` is required for `gh release create`. No other permissions granted.
- **Unsigned binaries.** Personal project, no Apple Developer account assumed; users handle quarantine.

## Release notes footer — `.github/release-notes-footer.md`

```markdown
---
### Installing

These binaries are unsigned. After download:
```bash
tar -xzf goove-vX.Y.Z-darwin-arm64.tar.gz
xattr -d com.apple.quarantine ./goove   # bypass Gatekeeper for unsigned binaries
./goove
```
```

## Linter config — `.golangci.yml`

```yaml
run:
  timeout: 3m

linters:
  enable:
    - errcheck       # un-checked errors
    - govet          # `go vet` integrated
    - ineffassign    # assignments whose value is never read
    - staticcheck    # the big bug-finder; supersedes gosimple/stylecheck
    - unused         # dead code
    - misspell       # typos in comments/strings
    - revive         # fast golint replacement, sensible defaults
```

### Design notes

- **`enable:` (not `enable-all:`)** — explicit, so a `golangci-lint` upgrade doesn't surprise us with new linters.
- **Deliberately skipped:** `gosec` (frequent false positives in app code), `gocritic` (opinionated style), `gocyclo`/`funlen` (style nits — review handles them better).
- **No exclusions** initially. If a rule fires often and isn't useful, we tune it then.

## Tool pinning approach

Versions are hardcoded in the `Makefile` as `GOLANGCI_LINT_VERSION` / `GOVULNCHECK_VERSION` variables. `make tools` does `go install …@$VERSION`.

**Considered alternative:** Go 1.24's `tool` directive in `go.mod` (`go get -tool …`, then `go tool golangci-lint run`). More idiomatic and version-locks via `go.sum`, but mixes non-runtime deps into `go.mod` and the toolchain is still rough in places. With only two tools and annual cadence, hardcoding is cheaper to reason about and easier to read at a glance. Migrate later if the tool list grows.

**Implementation note:** the `v1.62.2` pin shown above is a reasonable starting point but should be verified against the latest stable `golangci-lint` release at implementation time and bumped if a newer stable exists.

## README changes

Replace the existing "Development" section in `README.md`:

````markdown
## Development

```bash
make tools          # install pinned dev tools (one-time)
make help           # list all targets
make test           # unit tests
make ci             # everything CI runs (fmt, vet, lint, vuln, race tests, build)
make run            # run from source
make build          # produce a binary
```

Integration tests (hit real Music.app):

```bash
make test-integration
```
````

The links to design docs at the bottom of that section are preserved.

## Verification

We'll consider the work done when:

- `make ci` passes locally on a clean working tree.
- `make help` renders all targets with descriptions.
- `make tools` installs both tools cleanly into `$GOBIN`.
- A test PR pushed to a branch shows the GitHub `ci` job running and going green (with the new linter, vuln-check, race-test, fmt-check steps visible in the log).
- The release workflow is dry-run locally with `GOOS=darwin GOARCH=arm64 go build` and `GOOS=darwin GOARCH=amd64 go build` to confirm cross-compilation succeeds. The first real `gh release` happens when the user cuts a real version tag — we trust the happy path on the first run rather than burning a throwaway tag.

## Out of scope (for follow-up specs if desired)

- Coverage reporting and thresholds.
- Test result/lint annotations on PRs (`reviewdog`, `golangci-lint-action`).
- Multi-job parallelism in CI (`lint` + `test` jobs).
- Migrating tool pins to Go 1.24's `tool` directive.
- Code signing / notarisation of release binaries.
- Homebrew tap for `brew install goove`.
