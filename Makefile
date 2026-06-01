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
GO_WORKSPACE_MODULES := . api/gen
BUF_GO_GEN_TEMPLATE  := buf.go.gen.yaml
LINT_GOWORK          ?= auto

GOPATH    := $(shell go env GOPATH)
GOVERSION := $(shell go version)
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

# Tool versions — override to pin a specific version.
PROTOC_GEN_GO_VERSION        := latest
PROTOC_GEN_GO_GRPC_VERSION   := latest
PROTOC_GEN_GO_HTTP_VERSION   := v2.0.0-20260404020628-f149714c1d54
PROTOC_GEN_GO_ERRORS_VERSION := latest
PROTOC_GEN_OPENAPI_VERSION   := latest
PROTOC_GEN_VALIDATE_VERSION  := latest
KRATOS_VERSION               := latest
GNOSTIC_VERSION              := latest
BUF_VERSION                  := latest
GOLANGCI_LINT_VERSION        := latest
WIRE_VERSION                 := latest
ENT_VERSION                  := latest

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
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@$(PROTOC_GEN_GO_HTTP_VERSION)
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v2@$(PROTOC_GEN_GO_ERRORS_VERSION)
	@go install github.com/google/gnostic/cmd/protoc-gen-openapi@$(PROTOC_GEN_OPENAPI_VERSION)
	@go install github.com/envoyproxy/protoc-gen-validate@$(PROTOC_GEN_VALIDATE_VERSION)
	@go install ./cmd/protoc-gen-servora-authz
	@go install ./cmd/protoc-gen-servora-audit
	@go install ./cmd/protoc-gen-servora-mapper
	@go install ./cmd/protoc-gen-servora-authn
	@go install ./cmd/protoc-gen-servora-conf
	@echo "✓ Protoc plugins installed"

.PHONY: cli
cli: ## Install CLI tools (kratos, buf, golangci-lint, wire, ent, svr)
	@echo "==> Installing CLI tools..."
	@go install github.com/go-kratos/kratos/cmd/kratos/v2@$(KRATOS_VERSION)
	@go install github.com/google/gnostic@$(GNOSTIC_VERSION)
	@go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@go install github.com/google/wire/cmd/wire@$(WIRE_VERSION)
	@go install entgo.io/ent/cmd/ent@$(ENT_VERSION)
	@go install ./cmd/svr
	@echo "✓ CLI tools installed"

# ============================================================================
# DEPENDENCIES
# ============================================================================

.PHONY: dep
dep: ## Download Go module dependencies
	@$(foreach mod,$(GO_WORKSPACE_MODULES),echo "  $(mod)" && (cd $(ROOT_DIR)$(mod) && go mod download) && ) true

.PHONY: tidy
tidy: ## go mod tidy across modules and go work sync
	@echo "==> Tidying Go modules..."
	@$(foreach mod,$(GO_WORKSPACE_MODULES),echo "  $(mod)" && (cd $(ROOT_DIR)$(mod) && go mod tidy) && ) true
	@go work sync
	@echo "✓ Modules tidied"

# ============================================================================
# CODE GENERATION
# ============================================================================

.PHONY: gen
gen: ## Generate proto Go code
	@echo "==> Generating code via $(BUF_GO_GEN_TEMPLATE)..."
	@buf generate --template $(BUF_GO_GEN_TEMPLATE)
	@echo "✓ Code generated"

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
fmt: ## Run gofmt across modules
	@$(foreach mod,$(GO_WORKSPACE_MODULES),(cd $(ROOT_DIR)$(mod) && gofmt -w .) && ) true
	@echo "✓ Formatted"

.PHONY: vet
vet: ## Run go vet across modules
	@$(foreach mod,$(GO_WORKSPACE_MODULES),(cd $(ROOT_DIR)$(mod) && go vet ./...) && ) true

.PHONY: test
test: ## Run unit tests across modules (-short, no external deps)
	@$(foreach mod,$(GO_WORKSPACE_MODULES),echo "==> Testing $(mod)..." && (cd $(ROOT_DIR)$(mod) && go test -short ./...) && ) true

.PHONY: test.all
test.all: ## Run all tests including integration (needs Redis, etc.)
	@$(foreach mod,$(GO_WORKSPACE_MODULES),echo "==> Testing $(mod) (all)..." && (cd $(ROOT_DIR)$(mod) && go test ./...) && ) true

.PHONY: cover
cover: ## Run tests with coverage profile
	@$(foreach mod,$(GO_WORKSPACE_MODULES),(cd $(ROOT_DIR)$(mod) && go test -v ./... -coverprofile=coverage.out) && ) true

.PHONY: lint
lint: lint.go ## Run Go lint

.PHONY: lint.go
lint.go:
	@$(foreach mod,$(GO_WORKSPACE_MODULES),echo "==> Linting Go ($(mod), GOWORK=$(LINT_GOWORK))..." && (cd $(ROOT_DIR)$(mod) && GOWORK=$(LINT_GOWORK) golangci-lint run) && ) true

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
ci.lint: LINT_GOWORK=off
ci.lint: lint.go lint.proto ## CI-equivalent lint (GOWORK=off + proto lint)

# ============================================================================
# RELEASE
# ============================================================================

# Usage: make tag.api TAG=v0.2.0
.PHONY: tag.api
tag.api: ## Tag api/gen submodule (TAG=v0.x.y required)
ifndef TAG
	$(error TAG is required. Usage: make tag.api TAG=v0.x.y)
endif
	@git tag api/gen/$(TAG)
	@echo "✓ Tagged api/gen/$(TAG) (run 'git push --tags' to push)"

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
