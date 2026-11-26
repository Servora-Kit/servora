set minimum-version := "1.57.0"
set dotenv-load
set unstable
set lists

[windows]
set shell := ["powershell.exe", "-NoLogo", "-NoProfile", "-Command"]

ROOT_DIR := justfile_directory()
JUST := just_executable()
BUF_GO_GEN_TEMPLATE := env("BUF_GO_GEN_TEMPLATE", "buf.go.gen.yaml")
BUF_TS_GEN_TEMPLATE := env("BUF_TS_GEN_TEMPLATE", "buf.typescript.gen.yaml")
BUF_BREAKING_AGAINST := env("BUF_BREAKING_AGAINST", ".git#ref=origin/main")
PNPM := env("PNPM", "pnpm")
VERSION_ENV := env("VERSION", "")
VERSION := if VERSION_ENV != "" { VERSION_ENV } else { `git describe --tags --always --dirty` }
GIT_COMMIT := `git rev-parse HEAD`
GOVERSION := `go version`

# Show available recipes
[default]
help:
    @just --list

# Print build environment
env:
    @echo "ROOT_DIR:   {{ ROOT_DIR }}"
    @echo "VERSION:    {{ VERSION }}"
    @echo "GIT_COMMIT: {{ GIT_COMMIT }}"
    @echo "GOVERSION:  {{ GOVERSION }}"

# Install pinned CLI tools, git hooks, and frontend dependencies
[unix]
init:
    @mise install
    @prek install
    @"{{ JUST }}" --justfile "{{ join(ROOT_DIR, "justfile") }}" web-install

[windows]
init:
    @mise install
    @prek install
    @& "{{ JUST }}" --justfile "{{ join(ROOT_DIR, "justfile") }}" web-install

# Download Go module dependencies
[working-directory(ROOT_DIR)]
dep:
    @go mod download

# Run go mod tidy for the repository module
[working-directory(ROOT_DIR)]
tidy:
    @go mod tidy

# Generate proto Go code
[working-directory(ROOT_DIR)]
gen-go:
    @buf generate --template "{{ BUF_GO_GEN_TEMPLATE }}"

# Generate TypeScript code for Servora built-in proto
[working-directory(ROOT_DIR)]
gen-ts:
    @buf generate --template "{{ BUF_TS_GEN_TEMPLATE }}"

# Generate both Go and TypeScript proto code
gen: gen-go gen-ts

# Bootstrap base Go protobuf types before running custom plugins that import api/gen/go
[unix]
[working-directory(ROOT_DIR)]
_gen-go-pb:
    @buf generate --template '{"version":"v2","plugins":[{"local":["go","tool","protoc-gen-go"],"out":"api/gen/go","opt":"paths=source_relative"}]}'

[windows]
[working-directory(ROOT_DIR)]
_gen-go-pb:
    @buf generate --template '{\"version\":\"v2\",\"plugins\":[{\"local\":[\"go\",\"tool\",\"protoc-gen-go\"],\"out\":\"api/gen/go\",\"opt\":\"paths=source_relative\"}]}'

# Wipe generated Go and TypeScript directories and regenerate
gen-fresh: clean _gen-go-pb gen

# Regenerate under GOWORK=off and verify zero tracked or untracked drift
[unix]
[working-directory(ROOT_DIR)]
gen-check:
    #!/usr/bin/env bash
    set -euo pipefail
    GOWORK=off "{{ JUST }}" --justfile "{{ join(ROOT_DIR, "justfile") }}" gen
    paths=(
        'api/gen/go'
        'web/packages/proto-utils/src/gen'
    )
    status=0
    if ! git diff --exit-code --stat -- "${paths[@]}"; then
        echo "Generated files are out of date (tracked diffs above)." >&2
        status=1
    fi
    untracked=$(git ls-files --others --exclude-standard -- "${paths[@]}")
    if [ -n "$untracked" ]; then
        echo "Untracked generated files detected:" >&2
        printf '%s\n' "$untracked" >&2
        status=1
    fi
    exit "$status"

[script("powershell.exe", "-NoLogo", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File")]
[windows]
[working-directory(ROOT_DIR)]
gen-check:
    $ErrorActionPreference = "Stop"
    $env:GOWORK = "off"
    & "{{ JUST }}" --justfile "{{ join(ROOT_DIR, "justfile") }}" gen
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    $paths = @(
        "api/gen/go",
        "web/packages/proto-utils/src/gen"
    )
    $failed = $false
    git diff --exit-code --stat -- @paths
    if ($LASTEXITCODE -ne 0) {
        [Console]::Error.WriteLine("Generated files are out of date (tracked diffs above).")
        $failed = $true
    }
    $untracked = git ls-files --others --exclude-standard -- @paths
    if ($untracked) {
        [Console]::Error.WriteLine(("Untracked generated files detected:`n" + ($untracked -join "`n")))
        $failed = $true
    }
    if ($failed) { exit 1 }

# Remove generated Go and TypeScript directories
[unix]
clean:
    rm -rf -- "{{ join(ROOT_DIR, "api/gen/go") }}" "{{ join(ROOT_DIR, "web/packages/proto-utils/src/gen") }}"

[windows]
clean:
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue @("{{ join(ROOT_DIR, "api/gen/go") }}", "{{ join(ROOT_DIR, "web/packages/proto-utils/src/gen") }}")

# Run gofmt across the repository module
[working-directory(ROOT_DIR)]
fmt:
    @go fmt ./...

# Run go vet across the repository module
[working-directory(ROOT_DIR)]
vet:
    @go vet ./...

# Run unit tests (-short -race, no external deps)
[working-directory(ROOT_DIR)]
test *args:
    @go test -short -race -count=1 {{ args }} ./...

# Run all tests with race detector
[working-directory(ROOT_DIR)]
test-all *args:
    @go test -race -count=1 {{ args }} ./...

# Run the local SQLite Ent live contract (requires explicit DSN)
[unix]
[working-directory(ROOT_DIR)]
test-ent-sqlite:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "${SERVORA_ENT_SQLITE_DSN:-}" ]; then
        echo "SERVORA_ENT_SQLITE_DSN is required (use a dedicated fixture database)" >&2
        exit 2
    fi
    go test -count=1 -tags=integration ./contrib/db/entgo/crud -run '^TestSQLiteLiveContract$'

[script("powershell.exe", "-NoLogo", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File")]
[windows]
[working-directory(ROOT_DIR)]
test-ent-sqlite:
    $ErrorActionPreference = "Stop"
    if ([string]::IsNullOrEmpty($env:SERVORA_ENT_SQLITE_DSN)) {
        [Console]::Error.WriteLine("SERVORA_ENT_SQLITE_DSN is required (use a dedicated fixture database)")
        exit 2
    }
    go test -count=1 -tags=integration ./contrib/db/entgo/crud -run '^TestSQLiteLiveContract$'
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

# Run the local PostgreSQL Ent live contract (requires explicit DSN)
[unix]
[working-directory(ROOT_DIR)]
test-ent-postgres:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "${SERVORA_ENT_POSTGRES_DSN:-}" ]; then
        echo "SERVORA_ENT_POSTGRES_DSN is required (use a dedicated fixture database)" >&2
        exit 2
    fi
    go test -count=1 -tags=integration ./contrib/db/entgo/crud -run '^TestPostgresLiveContract$'

[script("powershell.exe", "-NoLogo", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File")]
[windows]
[working-directory(ROOT_DIR)]
test-ent-postgres:
    $ErrorActionPreference = "Stop"
    if ([string]::IsNullOrEmpty($env:SERVORA_ENT_POSTGRES_DSN)) {
        [Console]::Error.WriteLine("SERVORA_ENT_POSTGRES_DSN is required (use a dedicated fixture database)")
        exit 2
    }
    go test -count=1 -tags=integration ./contrib/db/entgo/crud -run '^TestPostgresLiveContract$'
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

# Run tests with coverage profile
[working-directory(ROOT_DIR)]
cover *args:
    @go test -v -race -count=1 -coverprofile=coverage.out {{ args }} ./...

# Run web, proto, and Go static checks
lint: web-check web-typecheck lint-proto lint-go

# Run golangci-lint across the repository module
[working-directory(ROOT_DIR)]
lint-go *args:
    @golangci-lint run {{ args }} ./...

# Run buf lint
[working-directory(ROOT_DIR)]
lint-proto *args:
    @buf lint {{ args }}

# Format proto files in place
[working-directory(ROOT_DIR)]
fmt-proto:
    @buf format -w

# Check proto file formatting
[working-directory(ROOT_DIR)]
fmt-proto-check *args:
    @buf format -d --exit-code {{ args }}

# Check proto breaking changes against BUF_BREAKING_AGAINST
[working-directory(ROOT_DIR)]
breaking *args:
    @buf breaking --exclude-imports --against "{{ BUF_BREAKING_AGAINST }}" {{ args }}

# Update BSR dependencies (buf.lock)
[working-directory(ROOT_DIR)]
bsr-update:
    @buf dep update

# Push proto to BSR (local fallback; CI handles pushes via buf-ci.yml)
[unix]
[working-directory(ROOT_DIR)]
bsr-push:
    #!/usr/bin/env bash
    set -euo pipefail
    git_tag="$(git tag --points-at HEAD 2>/dev/null | grep -E '^v[0-9]' | head -n 1 || true)"
    if [ -n "$git_tag" ]; then
        echo "==> Pushing to BSR with labels: $git_tag, main"
        buf push --exclude-unnamed --label "$git_tag" --label main
    else
        echo "==> No Git version tag on HEAD, pushing with label: main"
        buf push --exclude-unnamed --label main
    fi

[script("powershell.exe", "-NoLogo", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File")]
[windows]
[working-directory(ROOT_DIR)]
bsr-push:
    $ErrorActionPreference = "Stop"
    $gitTag = (git tag --points-at HEAD 2>$null | Where-Object { $_ -match '^v[0-9]' } | Select-Object -First 1)
    if ($gitTag) {
        Write-Output "==> Pushing to BSR with labels: $gitTag, main"
        buf push --exclude-unnamed --label $gitTag --label main
    } else {
        Write-Output "==> No Git version tag on HEAD, pushing with label: main"
        buf push --exclude-unnamed --label main
    }
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

# Install frontend workspace dependencies
[unix]
[working-directory("web")]
web-install:
    @"{{ PNPM }}" install --frozen-lockfile

[windows]
[working-directory("web")]
web-install:
    @& "{{ PNPM }}" install --frozen-lockfile

# Run Biome check across the frontend workspace
[unix]
[working-directory("web")]
web-check:
    @"{{ PNPM }}" run check

[windows]
[working-directory("web")]
web-check:
    @& "{{ PNPM }}" run check

# Typecheck @servora/proto-utils
[unix]
[working-directory("web/packages/proto-utils")]
web-typecheck:
    @"{{ PNPM }}" run typecheck

[windows]
[working-directory("web/packages/proto-utils")]
web-typecheck:
    @& "{{ PNPM }}" run typecheck

# Build @servora/proto-utils
[unix]
[working-directory("web/packages/proto-utils")]
web-build:
    @"{{ PNPM }}" run build

[windows]
[working-directory("web/packages/proto-utils")]
web-build:
    @& "{{ PNPM }}" run build

# Build, typecheck tests, and run @servora/proto-utils unit tests
[unix]
[working-directory("web/packages/proto-utils")]
web-test:
    @"{{ PNPM }}" run test

[windows]
[working-directory("web/packages/proto-utils")]
web-test:
    @& "{{ PNPM }}" run test
