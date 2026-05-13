# goove Devex Tooling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `Makefile` as the single source of truth for build/test/lint, expand CI to run `gofmt-check + golangci-lint + govulncheck + race tests + build`, and add a release workflow that publishes `darwin-arm64` + `darwin-amd64` tarballs on `v*` tag pushes.

**Architecture:** A small `Makefile` at the repo root holds every command developers and CI need. The CI workflow becomes a thin wrapper that does `setup-go → make tools → make ci`. A separate release workflow runs only on `v*` tags and uses the GitHub `gh` CLI to attach cross-compiled tarballs to a Release. Linter behaviour is governed by a minimal `.golangci.yml`.

**Tech Stack:** GNU make, GitHub Actions (`actions/checkout@v4`, `actions/setup-go@v5`), `golangci-lint`, `govulncheck`, `gofmt`, `go test -race`. Spec: `docs/superpowers/specs/2026-05-06-devex-tooling-design.md`.

---

## File Structure

```
goove/
├── Makefile                                  # T3: new — all build/test/lint targets
├── .golangci.yml                             # T2: new — minimal lint config
├── .gitignore                                # T4: modify — add /main and dist/
├── README.md                                 # T7: modify — Development section rewritten
├── .github/
│   ├── release-notes-footer.md               # T9: new — install instructions appended to releases
│   └── workflows/
│       ├── ci.yml                            # T8: replace — call `make ci` instead of inline go commands
│       └── release.yml                       # T10: new — tag-driven Mac binary release
└── docs/superpowers/
    ├── specs/2026-05-06-devex-tooling-design.md   # spec (existing)
    └── plans/2026-05-06-devex-tooling.md          # this plan
```

No source-code changes are expected. If the new linters fire on existing Go files, those changes happen in Task 6 inside `internal/` — the plan does not pre-list those locations because they're discovered, not designed.

## Naming and contract

| Symbol | Definition |
|---|---|
| `make tools` | Installs `golangci-lint` (pinned via `GOLANGCI_LINT_VERSION`) and `govulncheck` (`@latest`) into `$GOBIN`. Must succeed on a fresh dev machine without prior setup. |
| `make ci` | Aggregate target: depends on `fmt-check vet lint vuln test-race build`. Exit code 0 = the same checks GitHub Actions ran. Used directly by `ci.yml`. |
| `make fmt-check` | Runs `gofmt -l .` and exits 1 if any file is unformatted. Does not modify the tree. |
| `make fmt` | Runs `gofmt -w .`. Modifies the tree. Never called by CI. |
| `GOLANGCI_LINT_VERSION` | Make variable holding the pinned `golangci-lint` version (e.g. `v1.62.2`). Override-able from the environment. |
| `GOVULNCHECK_VERSION` | Make variable for the `govulncheck` version. Default `latest`. |
| `.github/release-notes-footer.md` | Static markdown appended to every GitHub Release body via `gh release create --notes-file`. Documents the `xattr -d com.apple.quarantine` Gatekeeper bypass for unsigned binaries. |
| `release.yml` job trigger | `on.push.tags: ['v*']` only. Does not fire on branch pushes or PRs. |
| Tarball naming | `goove-${{ github.ref_name }}-darwin-${ARCH}.tar.gz`, e.g. `goove-v0.1.0-darwin-arm64.tar.gz`. Inside the tarball the binary is plain `goove`. |

---

## Phase 1 — Bootstrap

### Task 1: Create feature branch and verify clean starting state

**No files modified.**

- [ ] **Step 1: Create the feature branch from main**

Run:
```bash
git checkout main
git checkout -b feature/devex-tooling
```

DO NOT run `git pull`. Local `main` carries the spec commit (`b6190dc`) which has not yet been pushed.

- [ ] **Step 2: Confirm the spec is present and the working tree is clean**

Run:
```bash
git status
ls docs/superpowers/specs/2026-05-06-devex-tooling-design.md
```

Expected: `git status` shows the branch up to date with `feature/devex-tooling` and a clean tree (untracked `.claude/` and `main` binary are pre-existing — leave them alone for now). The spec file exists.

- [ ] **Step 3: Confirm Go and gh tooling are available**

Run:
```bash
go version
gh --version
```

Expected: `go version` returns `go1.24` or higher (we develop on whatever the user has; CI pins `1.24`). `gh` is installed and reports a version. If `gh` is missing, install via `brew install gh` before proceeding (the release workflow uses it; local dry-runs in Task 11 also need it indirectly).

---

## Phase 2 — Local tooling (Makefile + linter)

### Task 2: Add `.golangci.yml`

**Files:**
- Create: `.golangci.yml`

- [ ] **Step 1: Create the linter config**

Write `.golangci.yml` with exactly this content:

```yaml
run:
  timeout: 3m

linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - misspell
    - revive
```

Notes:
- `enable:` (not `enable-all:`) keeps the linter set explicit.
- No exclusions yet. We'll discover and tune in Task 6.

- [ ] **Step 2: Commit**

```bash
git add .golangci.yml
git commit -m "build: add minimal golangci-lint config"
```

### Task 3: Add the `Makefile`

**Files:**
- Create: `Makefile`

- [ ] **Step 1: Verify the latest stable golangci-lint version**

Run:
```bash
gh api repos/golangci/golangci-lint/releases/latest --jq .tag_name
```

Expected: a tag like `v1.62.2` or newer. Note this version — you'll plug it into `GOLANGCI_LINT_VERSION` in step 2. If the network call fails, fall back to `v1.62.2` (the spec's reference pin).

- [ ] **Step 2: Create the Makefile**

Write `Makefile` at the repo root with this content (substitute the version from step 1 into `GOLANGCI_LINT_VERSION`):

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

**IMPORTANT:** Make recipes require **tab** indentation, not spaces. After saving, run `cat -A Makefile | grep -E '^\^I' | head -3` and confirm you see `^I` (tab) at the start of recipe lines. If you see spaces, the Makefile will fail with `*** missing separator. Stop.`

- [ ] **Step 3: Verify `make help` works**

Run:
```bash
make help
```

Expected: a list of targets with descriptions, each line cyan-coloured. No errors. The targets listed should include: `help`, `build`, `run`, `install`, `test`, `test-race`, `test-integration`, `vet`, `fmt`, `fmt-check`, `lint`, `vuln`, `tools`, `ci`, `clean`.

- [ ] **Step 4: Commit**

```bash
git add Makefile
git commit -m "build: add Makefile as single source of truth for build/test/lint"
```

### Task 4: Update `.gitignore`

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: Inspect current `.gitignore`**

Run:
```bash
cat .gitignore
```

Confirm it ends with `# goove built binary` followed by `/goove`. We'll add `/main` (the stale binary at the repo root) and `dist/` (created by the release workflow's local dry-run in Task 11).

- [ ] **Step 2: Append to `.gitignore`**

Add these lines at the end of `.gitignore`:

```
/main

# Release workflow staging
dist/
```

- [ ] **Step 3: Confirm both untracked files are now ignored**

Run:
```bash
git status --ignored | grep -E 'main|dist|goove'
```

Expected: `main` and `goove` appear under "Ignored files". `dist/` will not appear because it doesn't exist yet — that's fine; the rule is in place.

- [ ] **Step 4: Commit**

```bash
git add .gitignore
git commit -m "build: ignore stale /main binary and release dist/ dir"
```

### Task 5: Install dev tools

**No files modified.**

- [ ] **Step 1: Run `make tools`**

Run:
```bash
make tools
```

Expected: two `go install` invocations succeed. `golangci-lint` may take 30-60s on first run. `govulncheck` is faster.

- [ ] **Step 2: Verify both binaries are on PATH**

Run:
```bash
which golangci-lint
which govulncheck
golangci-lint --version
govulncheck -version
```

Expected: both commands print a path under `$HOME/go/bin/` (or wherever `$GOBIN` points), and both report a version. If `which` fails, ensure `$HOME/go/bin` (or `$GOBIN`) is on `$PATH`. The user's shell profile is the right place to fix that — note it but don't modify shell rc files.

### Task 6: Run `make ci` locally and fix discovered issues

**No files modified up front. Issues found in this task are fixed in-place across `internal/` and committed per category (formatting / lint / vuln / race) so each fix has a clean diff.**

- [ ] **Step 1: Run `make fmt-check`**

Run:
```bash
make fmt-check
```

If it passes, skip to step 2. If it fails:
1. Run `make fmt` to apply fixes.
2. Run `git diff` to inspect the changes — they should only be whitespace.
3. Commit: `git commit -am "style: gofmt -w ."`
4. Re-run `make fmt-check` to confirm clean.

- [ ] **Step 2: Run `make vet`**

Run:
```bash
make vet
```

If it passes, skip to step 3. If it fails: read each reported issue, fix it in the named file (these are correctness bugs, not style — fix don't suppress), then commit per concern: `git commit -am "fix: <concise description of vet finding>"`. Re-run until clean.

- [ ] **Step 3: Run `make lint`**

Run:
```bash
make lint
```

If it passes, skip to step 4. If it fails, sort findings into three buckets:

1. **Real bugs** (e.g. `errcheck` on a non-trivial `Close()`, `unused` flagging genuinely dead code, `staticcheck` correctness check): fix the code, commit per fix or per category. Example commit: `git commit -am "fix: handle io.Close errors in <file>"`.
2. **Intentional ignores at the call site** (e.g. `defer f.Close()` where the close-error is genuinely uninteresting): suppress narrowly with `//nolint:errcheck // close on read-only file, error not actionable` on the offending line. Use this sparingly — every nolint costs a reader's attention.
3. **Linter-wide noise** (a rule fires dozens of times across the codebase and the noise isn't actionable): edit `.golangci.yml` to disable that one rule under `linters: disable:` or add an `issues.exclude-rules:` entry scoped to the noisy path. Document why in a `# comment` next to the rule. Commit: `git commit -am "build: tune golangci-lint for <reason>"`.

After each batch of fixes, re-run `make lint`. Repeat until clean. **Do not** silently disable everything to make the linter green — if a category is genuinely a wash, that's a signal worth surfacing rather than burying.

- [ ] **Step 4: Run `make vuln`**

Run:
```bash
make vuln
```

If it passes, skip to step 5. If it fails: each finding names a vulnerable module. Run `go get <module>@latest` for each, then `go mod tidy`. Commit: `git commit -am "deps: bump <module> for <CVE-ID>"`. If a finding is in the standard library (rare), bump the `go` directive in `go.mod` to a version that includes the fix.

- [ ] **Step 5: Run `make test-race`**

Run:
```bash
make test-race
```

If it passes, skip to step 6. If it fails: race-detector failures are real bugs (unsynchronised concurrent access to shared state). The `-race` output points at the racing goroutines and the offending memory address. Fix by adding the right synchronisation (mutex, channel, atomic) — do not suppress. Commit per fix: `git commit -am "fix: data race in <component>"`.

- [ ] **Step 6: Run `make build`**

Run:
```bash
make build
```

Expected: produces `./goove`. If this fails after the previous steps passed, there's a build-tag-only error somewhere — read the message and fix.

- [ ] **Step 7: Run `make ci` end-to-end**

Run:
```bash
make ci
```

Expected: every chained target passes. The terminal output ends with the build step succeeding (no explicit success line — exit code 0 is the contract). If anything fails here that didn't fail in steps 1–6, you missed a re-run somewhere; revisit.

- [ ] **Step 8: Confirm working tree is clean**

Run:
```bash
git status
git log --oneline feature/devex-tooling ^main
```

Expected: clean tree (the new `goove` binary is gitignored). The feature branch carries the spec commit and several new commits from Tasks 2-6.

---

## Phase 3 — Documentation

### Task 7: Update README Development section

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Read the current Development section**

Run:
```bash
grep -n "## Development" README.md
```

Expected: a single line number (currently around line 114-115). The section runs until `## License`.

- [ ] **Step 2: Replace the Development section**

Use the `Read` tool to load `README.md`, then use `Edit` with the existing Development section as `old_string` and this exact text as `new_string` (the outer quad-backticks below are only to escape the nested fences in this plan — they are not part of the file content):

`````markdown
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

The design lives in [`docs/superpowers/specs/2026-04-30-goove-mvp-design.md`](docs/superpowers/specs/2026-04-30-goove-mvp-design.md).
The plan it was built against lives in [`docs/superpowers/plans/2026-04-30-goove-mvp.md`](docs/superpowers/plans/2026-04-30-goove-mvp.md).
The TUI overhaul (LazyGit-inspired multi-panel layout) is specced in
[`docs/superpowers/specs/2026-05-04-tui-overhaul-design.md`](docs/superpowers/specs/2026-05-04-tui-overhaul-design.md)
and planned in
[`docs/superpowers/plans/2026-05-04-tui-overhaul.md`](docs/superpowers/plans/2026-05-04-tui-overhaul.md).
`````

The design-doc links are preserved verbatim from the previous version. The blank line before `## License` (which remains in the file) should also be preserved.

- [ ] **Step 3: Verify the README still renders sensibly**

Run:
```bash
grep -n "## " README.md
```

Expected: section headings appear in this order: `# goove`, `## Install`, `## Run`, `## Keys`, `## CLI commands`, `## Logs`, `## Development`, `## License`. No duplicate `## Development`, no orphan `## License`.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: README Development section uses make targets"
```

---

## Phase 4 — CI workflow

### Task 8: Replace `.github/workflows/ci.yml`

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Replace the file's entire contents**

Overwrite `.github/workflows/ci.yml` with:

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

- [ ] **Step 2: Sanity-check the YAML locally**

Run:
```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"
```

Expected: no output (valid YAML). If `python3` is unavailable, skip this step — the next push will surface any YAML errors.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: replace inline go commands with 'make ci'"
```

### Task 9: Push branch and verify CI passes

**No files modified.**

- [ ] **Step 1: Push the branch and the spec commit**

Run:
```bash
git push -u origin feature/devex-tooling
```

Note: this push includes both the spec commit (`b6190dc` from local `main`'s tip-before-branching) and all the feature-branch commits. The spec commit will end up in `feature/devex-tooling` only; it lands on `main` when this branch merges.

- [ ] **Step 2: Open a PR**

Run:
```bash
gh pr create --title "Add Makefile and expand CI tooling" --body "$(cat <<'EOF'
## Summary
- Add `Makefile` as the single source of truth for build/test/lint commands.
- Expand CI with `gofmt-check`, `golangci-lint`, `govulncheck`, race detector.
- Add release workflow that publishes `darwin-arm64` + `darwin-amd64` tarballs on `v*` tag pushes.
- README's Development section now points at `make` targets.

Spec: `docs/superpowers/specs/2026-05-06-devex-tooling-design.md`
Plan: `docs/superpowers/plans/2026-05-06-devex-tooling.md`

## Test plan
- [ ] CI green on this PR (the new workflow is validating itself).
- [ ] `make help`, `make ci` work locally for a new contributor following the README.
- [ ] Release workflow validated by the next real `v*` tag (no throwaway tag).
EOF
)"
```

- [ ] **Step 3: Wait for CI to complete and verify it passes**

Run:
```bash
gh pr checks --watch
```

Expected: the `ci` job completes with `pass`. If it fails:
1. Run `gh run view --log-failed` to see the offending step's output.
2. Common failure modes:
   - **`make tools` fails**: network or GOBIN/PATH issue on the runner. Read the error; setup-go puts `$HOME/go/bin` on PATH automatically, so this is rare.
   - **`make fmt-check` fails on CI but not locally**: line-ending or hidden-file mismatch. Re-run `make fmt && git status` locally; if the diff is empty, look for files added on the runner only (very rare).
   - **`make lint` fails on CI but not locally**: pinned version mismatch. Confirm `GOLANGCI_LINT_VERSION` in the Makefile matches the version `make tools` actually installs.
   - **`make test-race` fails on CI but not locally**: a real race that only surfaces under the runner's CPU count. Read the race report, fix, push.
3. Fix and push another commit; CI re-runs automatically.

Do not merge yet — Phase 5 still has work to add.

---

## Phase 5 — Release workflow

### Task 10: Add `.github/release-notes-footer.md`

**Files:**
- Create: `.github/release-notes-footer.md`

- [ ] **Step 1: Create the footer file**

Write `.github/release-notes-footer.md` with this content:

````markdown
---
### Installing

These binaries are unsigned. After download:
```bash
tar -xzf goove-vX.Y.Z-darwin-arm64.tar.gz
xattr -d com.apple.quarantine ./goove   # bypass Gatekeeper for unsigned binaries
./goove
```
````

The leading `---` produces a horizontal rule separating the auto-generated PR notes from the install instructions.

- [ ] **Step 2: Commit**

```bash
git add .github/release-notes-footer.md
git commit -m "ci: add release notes footer with unsigned-binary install steps"
```

### Task 11: Add `.github/workflows/release.yml`

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Create the workflow**

Write `.github/workflows/release.yml`:

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

- [ ] **Step 2: Sanity-check the YAML**

Run:
```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"
```

Expected: no output. If `python3` is unavailable, skip.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: add release workflow for darwin tarballs on v* tags"
```

### Task 12: Local dry-run of cross-compilation

**No files modified.**

- [ ] **Step 1: Build for darwin-arm64**

Run:
```bash
mkdir -p dist
GOOS=darwin GOARCH=arm64 go build -o dist/goove ./cmd/goove
file dist/goove
```

Expected: `file` reports `Mach-O 64-bit executable arm64`. Confirm size is non-trivial (~5MB).

- [ ] **Step 2: Build for darwin-amd64**

Run:
```bash
GOOS=darwin GOARCH=amd64 go build -o dist/goove ./cmd/goove
file dist/goove
```

Expected: `file` reports `Mach-O 64-bit executable x86_64`.

- [ ] **Step 3: Verify tarball packaging works**

Run:
```bash
tar -czf /tmp/goove-test-darwin-amd64.tar.gz -C dist goove
tar -tzf /tmp/goove-test-darwin-amd64.tar.gz
```

Expected: the tarball lists exactly one entry: `goove`. No leading `dist/` prefix (the `-C dist` flag handled that).

- [ ] **Step 4: Clean up**

Run:
```bash
rm -rf dist /tmp/goove-test-darwin-amd64.tar.gz
git status
```

Expected: clean working tree (the `dist/` rule from Task 4 keeps it untracked anyway).

### Task 13: Push remaining commits and confirm CI still green

**No files modified.**

- [ ] **Step 1: Push**

Run:
```bash
git push
```

This pushes Tasks 10–12's commits to the existing PR.

- [ ] **Step 2: Wait for CI**

Run:
```bash
gh pr checks --watch
```

Expected: the `ci` job re-runs and passes. The new `release.yml` does **not** run on this push (it's tag-triggered) — that's correct.

- [ ] **Step 3: Inspect the PR diff**

Run:
```bash
gh pr view --web
```

Spot-check: the PR contains the spec, the plan, the new `Makefile`, `.golangci.yml`, both workflows, the release notes footer, the `.gitignore` updates, the README rewrite, and (if Task 6 surfaced any) lint/format/race fixes inside `internal/`.

The PR is ready for the user to review and merge. The release workflow's first real exercise will be the next `v*` tag the user pushes — no throwaway tag is needed.

---

## Done criteria

The plan is complete when **all** are true:

- [ ] `make ci` passes locally on `feature/devex-tooling` with a clean working tree.
- [ ] `make help` lists every target with a description.
- [ ] CI green on the open PR.
- [ ] `release.yml` is in place and YAML-valid (it does not run until a `v*` tag is pushed).
- [ ] README's Development section references `make` targets only.
- [ ] No new untracked files at the repo root after a clean build cycle.
