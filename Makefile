# ============================================================================
# Makefile for servora framework
# ============================================================================

ifneq (,$(wildcard .env))
    include .env
    export
endif

# ============================================================================
# VARIABLES
# ============================================================================

ROOT_DIR             := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))
BUF_GO_GEN_TEMPLATE  := buf.go.gen.yaml
BUF_TS_GEN_TEMPLATE  := buf.typescript.gen.yaml

GOPATH    := $(shell go env GOPATH)
GOVERSION := $(shell go version)
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

# Tool versions — override to pin a specific version.
PROTOC_GEN_GO_VERSION        := latest
PROTOC_GEN_GO_GRPC_VERSION   := latest
PROTOC_GEN_GO_HTTP_VERSION   := latest
PROTOC_GEN_OPENAPI_VERSION   := latest
PROTOC_GEN_VALIDATE_VERSION  := latest
PROTOC_GEN_GO_REDACT_VERSION := latest
KRATOS_VERSION               := latest
GNOSTIC_VERSION              := latest
BUF_VERSION                  := latest
GOLANGCI_LINT_VERSION        := latest
WIRE_VERSION                 := latest
ENT_VERSION                  := latest
API_LINTER_VERSION           := latest

# ============================================================================
# META
# ============================================================================

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_.-]+:.*?## / { printf "  %-16s  %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: env
env: ## Print build environment
	@echo "ROOT_DIR:   $(ROOT_DIR)"
	@echo "VERSION:    $(VERSION)"
	@echo "GIT_COMMIT: $(GIT_COMMIT)"
	@echo "GOVERSION:  $(GOVERSION)"

# ============================================================================
# TOOLCHAIN
# ============================================================================

.PHONY: init
init: plugin cli ## Install protoc plugins and CLI tools

.PHONY: plugin
plugin: ## Install protoc-gen-* plugins (third-party + servora)
	@echo "==> Installing protoc plugins..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v3@$(PROTOC_GEN_GO_HTTP_VERSION)
	@go install ./cmd/protoc-gen-typescript-http
	@go install ./cmd/protoc-gen-go-errors
	@go install github.com/google/gnostic/cmd/protoc-gen-openapi@$(PROTOC_GEN_OPENAPI_VERSION)
	@go install github.com/envoyproxy/protoc-gen-validate@$(PROTOC_GEN_VALIDATE_VERSION)
	@go install ./cmd/protoc-gen-redact
	@go install ./cmd/protoc-gen-servora-audit
	@go install ./cmd/protoc-gen-servora-conf
	@go install ./cmd/protoc-gen-servora-crud
	@echo "✓ Protoc plugins installed"

.PHONY: cli
cli: ## Install CLI tools (kratos, buf, golangci-lint, wire, ent, svr)
	@echo "==> Installing CLI tools..."
	@go install github.com/go-kratos/kratos/cmd/kratos/v3@$(KRATOS_VERSION)
	@go install github.com/google/gnostic@$(GNOSTIC_VERSION)
	@go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@go install github.com/googleapis/api-linter/cmd/api-linter@$(API_LINTER_VERSION)
	@go install github.com/google/wire/cmd/wire@$(WIRE_VERSION)
	@go install entgo.io/ent/cmd/ent@$(ENT_VERSION)
	@go install ./cmd/svr
	@echo "✓ CLI tools installed"

# ============================================================================
# DEPENDENCIES
# ============================================================================

.PHONY: dep
dep: ## Download Go module dependencies
	@go mod download

.PHONY: tidy
tidy: ## Run go mod tidy for the repository module
	@echo "==> Tidying Go module..."
	@go mod tidy
	@echo "✓ Module tidied"

# ============================================================================
# CODE GENERATION
# ============================================================================

.PHONY: gen
gen: ## Generate proto Go code
	@echo "==> Generating code via $(BUF_GO_GEN_TEMPLATE)..."
	@buf generate --template $(BUF_GO_GEN_TEMPLATE)
	@echo "✓ Code generated"

.PHONY: gen.ts
gen.ts: ## Generate TypeScript code for Servora built-in proto
	@echo "==> Generating TypeScript via $(BUF_TS_GEN_TEMPLATE)..."
	@buf generate --template $(BUF_TS_GEN_TEMPLATE)
	@echo "✓ TypeScript code generated"

.PHONY: web.install
web.install: ## Install frontend package dependencies
	@cd web && pnpm install

.PHONY: web.typecheck
web.typecheck: ## Typecheck @servora/proto-utils
	@cd web/packages/proto-utils && pnpm run typecheck

.PHONY: web.build
web.build: ## Build @servora/proto-utils
	@cd web/packages/proto-utils && pnpm run build

.PHONY: gen.fresh
gen.fresh: clean gen ## Wipe api/gen/go and regenerate (use after proto rename/deletion or plugin removal)

.PHONY: bsr.update
bsr.update: ## Update BSR dependencies (buf.lock)
	@buf dep update
	@echo "✓ BSR dependencies updated"

.PHONY: clean
clean: ## Remove generated code
	@rm -rf api/gen/go
	@echo "✓ Cleaned"

# ============================================================================
# QUALITY
# ============================================================================

.PHONY: fmt
fmt: ## Run gofmt across the repository module
	@go fmt ./...
	@echo "✓ Formatted"

.PHONY: vet
vet: ## Run go vet across the repository module
	@go vet ./...

.PHONY: test
test: ## Run unit tests (-short, no external deps)
	@go test -short ./...

.PHONY: test.all
test.all: ## Run all tests including integration (needs Redis, etc.)
	@go test ./...

.PHONY: test.ent.sqlite
test.ent.sqlite: export SERVORA_ENT_SQLITE_DSN := $(SERVORA_ENT_SQLITE_DSN)
test.ent.sqlite: ## Run the local SQLite Ent live contract (requires explicit DSN)
	@test -n "$$SERVORA_ENT_SQLITE_DSN" || { echo "SERVORA_ENT_SQLITE_DSN is required (use a dedicated fixture database)" >&2; exit 2; }
	@go test -count=1 -tags=integration ./contrib/db/entgo/crud -run '^TestSQLiteLiveContract$$'

.PHONY: test.ent.postgres
test.ent.postgres: export SERVORA_ENT_POSTGRES_DSN := $(SERVORA_ENT_POSTGRES_DSN)
test.ent.postgres: ## Run the local PostgreSQL Ent live contract (requires explicit DSN)
	@test -n "$$SERVORA_ENT_POSTGRES_DSN" || { echo "SERVORA_ENT_POSTGRES_DSN is required (use a dedicated fixture database)" >&2; exit 2; }
	@go test -count=1 -tags=integration ./contrib/db/entgo/crud -run '^TestPostgresLiveContract$$'

.PHONY: cover
cover: ## Run tests with coverage profile
	@go test -v ./... -coverprofile=coverage.out

.PHONY: lint
lint: lint.go lint.proto ## Run Go and Proto lint

.PHONY: lint.go
lint.go:
	@golangci-lint run ./...

.PHONY: lint.proto
lint.proto: ## Run buf lint
	@buf lint
	@echo "✓ Proto lint passed"

.PHONY: fmt.proto
fmt.proto: ## Format proto files (buf format -w)
	@buf format -w
	@echo "✓ Proto formatted"

# CI-equivalent path: disable Go workspace, then lint Go + proto.
.PHONY: ci.lint
ci.lint: export GOWORK := off
ci.lint: lint.go lint.proto ## CI-equivalent lint (GOWORK=off + proto lint)

# ============================================================================
# RELEASE
# ============================================================================

.PHONY: bsr.push
# 日常 BSR 推送已交给 .github/workflows/buf-ci.yml；此 target 仅作本地预演/应急使用
bsr.push: ## Push proto to BSR (local fallback; CI handles daily pushes via buf-ci.yml)
	@GIT_TAG=$$(git tag --points-at HEAD 2>/dev/null | grep -E '^v[0-9]' | head -1); \
	if [ -n "$$GIT_TAG" ]; then \
		echo "==> Pushing to BSR with labels: $$GIT_TAG, main"; \
		buf push --exclude-unnamed --label "$$GIT_TAG" --label main; \
	else \
		echo "==> No Git version tag on HEAD, pushing with label: main"; \
		buf push --exclude-unnamed --label main; \
	fi
	@echo "✓ Proto pushed to BSR"
