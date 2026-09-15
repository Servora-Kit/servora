set dotenv-load
set dotenv-override
set export

ROOT_DIR := justfile_directory() + "/"
BUF_GO_GEN_TEMPLATE := "buf.go.gen.yaml"
BUF_TS_GEN_TEMPLATE := "buf.typescript.gen.yaml"

GOPATH := `go env GOPATH`
GOVERSION := `go version`
VERSION := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
GIT_COMMIT := `git rev-parse HEAD 2>/dev/null || echo "unknown"`

# Tool versions — override to pin a specific version.
PROTOC_GEN_GO_VERSION := "latest"
PROTOC_GEN_GO_GRPC_VERSION := "latest"
PROTOC_GEN_GO_HTTP_VERSION := "latest"
PROTOC_GEN_OPENAPI_VERSION := "latest"
PROTOC_GEN_VALIDATE_VERSION := "latest"
PROTOC_GEN_GO_REDACT_VERSION := "latest"
KRATOS_VERSION := "latest"
GNOSTIC_VERSION := "latest"
BUF_VERSION := "latest"
GOLANGCI_LINT_VERSION := "latest"
WIRE_VERSION := "latest"
ENT_VERSION := "latest"
API_LINTER_VERSION := "latest"

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

# Install protoc plugins and CLI tools
init: plugin && cli

# Install protoc-gen-* plugins (third-party + servora)
plugin:
    @echo "==> Installing protoc plugins..."
    @go install google.golang.org/protobuf/cmd/protoc-gen-go@{{ PROTOC_GEN_GO_VERSION }}
    @go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@{{ PROTOC_GEN_GO_GRPC_VERSION }}
    @go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v3@{{ PROTOC_GEN_GO_HTTP_VERSION }}
    @go install ./cmd/protoc-gen-typescript-http
    @go install ./cmd/protoc-gen-go-errors
    @go install github.com/google/gnostic/cmd/protoc-gen-openapi@{{ PROTOC_GEN_OPENAPI_VERSION }}
    @go install github.com/envoyproxy/protoc-gen-validate@{{ PROTOC_GEN_VALIDATE_VERSION }}
    @go install ./cmd/protoc-gen-redact
    @go install ./cmd/protoc-gen-servora-audit
    @go install ./cmd/protoc-gen-servora-conf
    @go install ./cmd/protoc-gen-servora-crud
    @echo "✓ Protoc plugins installed"

# Install CLI tools (kratos, buf, golangci-lint, wire, ent, svr)
cli:
    @echo "==> Installing CLI tools..."
    @go install github.com/go-kratos/kratos/cmd/kratos/v3@{{ KRATOS_VERSION }}
    @go install github.com/google/gnostic@{{ GNOSTIC_VERSION }}
    @go install github.com/bufbuild/buf/cmd/buf@{{ BUF_VERSION }}
    @go install github.com/golangci/golangci-lint/cmd/golangci-lint@{{ GOLANGCI_LINT_VERSION }}
    @go install github.com/googleapis/api-linter/cmd/api-linter@{{ API_LINTER_VERSION }}
    @go install github.com/google/wire/cmd/wire@{{ WIRE_VERSION }}
    @go install entgo.io/ent/cmd/ent@{{ ENT_VERSION }}
    @go install ./cmd/svr
    @echo "✓ CLI tools installed"

# Download Go module dependencies
dep:
    @go mod download

# Run go mod tidy for the repository module
tidy:
    @echo "==> Tidying Go module..."
    @go mod tidy
    @echo "✓ Module tidied"

# Generate proto Go code
gen:
    @echo "==> Generating code via {{ BUF_GO_GEN_TEMPLATE }}..."
    @buf generate --template {{ BUF_GO_GEN_TEMPLATE }}
    @echo "✓ Code generated"

# Generate TypeScript code for Servora built-in proto
gen-ts:
    @echo "==> Generating TypeScript via {{ BUF_TS_GEN_TEMPLATE }}..."
    @buf generate --template {{ BUF_TS_GEN_TEMPLATE }}
    @echo "✓ TypeScript code generated"

# Install frontend package dependencies
[working-directory("web")]
web-install:
    @pnpm install

# Typecheck @servora/proto-utils
[working-directory("web/packages/proto-utils")]
web-typecheck:
    @pnpm run typecheck

# Build @servora/proto-utils
[working-directory("web/packages/proto-utils")]
web-build:
    @pnpm run build

# Test @servora/proto-utils
[working-directory("web/packages/proto-utils")]
web-test:
    @pnpm run test

# Wipe api/gen/go and regenerate after incompatible proto changes
gen-fresh: clean && gen

# Update BSR dependencies (buf.lock)
bsr-update:
    @buf dep update
    @echo "✓ BSR dependencies updated"

# Remove generated code
clean:
    @rm -rf api/gen/go
    @echo "✓ Cleaned"

# Run gofmt across the repository module
fmt:
    @go fmt ./...
    @echo "✓ Formatted"

# Run go vet across the repository module
vet:
    @go vet ./...

# Run unit tests (-short, no external deps)
test:
    @go test -short ./...

# Run all tests without build tags; existing external-service skips still apply
test-all:
    @go test ./...

# Run the local SQLite Ent live contract (requires explicit DSN)
test-ent-sqlite:
    #!/usr/bin/env sh
    set -eu
    if [ -z "${SERVORA_ENT_SQLITE_DSN:-}" ]; then
        echo "SERVORA_ENT_SQLITE_DSN is required (use a dedicated fixture database)" >&2
        exit 2
    fi
    go test -count=1 -tags=integration ./contrib/db/entgo/crud -run '^TestSQLiteLiveContract$'

# Run the local PostgreSQL Ent live contract (requires explicit DSN)
test-ent-postgres:
    #!/usr/bin/env sh
    set -eu
    if [ -z "${SERVORA_ENT_POSTGRES_DSN:-}" ]; then
        echo "SERVORA_ENT_POSTGRES_DSN is required (use a dedicated fixture database)" >&2
        exit 2
    fi
    go test -count=1 -tags=integration ./contrib/db/entgo/crud -run '^TestPostgresLiveContract$'

# Run tests with coverage profile
cover:
    @go test -v ./... -coverprofile=coverage.out

# Run Go and Proto lint
lint: lint-go && lint-proto

# Run golangci-lint across the repository module
lint-go:
    @golangci-lint run ./...

# Run buf lint
lint-proto:
    @buf lint
    @echo "✓ Proto lint passed"

# Format proto files (buf format -w)
fmt-proto:
    @buf format -w
    @echo "✓ Proto formatted"

# Run CI-equivalent lint (GOWORK=off + proto lint)
ci-lint:
    #!/usr/bin/env sh
    set -eu
    GOWORK=off just lint-go
    just lint-proto

# Push proto to BSR (local fallback; CI handles daily pushes via buf-ci.yml)
bsr-push:
    #!/usr/bin/env sh
    set -eu
    GIT_TAG=$(git tag --points-at HEAD 2>/dev/null | grep -E '^v[0-9]' | head -1)
    if [ -n "$GIT_TAG" ]; then
        echo "==> Pushing to BSR with labels: $GIT_TAG, main"
        buf push --exclude-unnamed --label "$GIT_TAG" --label main
    else
        echo "==> No Git version tag on HEAD, pushing with label: main"
        buf push --exclude-unnamed --label main
    fi
    echo "✓ Proto pushed to BSR"
