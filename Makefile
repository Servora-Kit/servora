# ============================================================================
# Makefile for servora framework
# ============================================================================

ifeq ($(OS),Windows_NT)
    IS_WINDOWS := 1
endif

ifneq (,$(wildcard .env))
    include .env
    export
endif

# ============================================================================
# VARIABLES & CONFIGURATION
# ============================================================================

CURRENT_DIR := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
ROOT_DIR    := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))
API_DIR     := api
PKG_DIR     := pkg
GO_WORKSPACE_MODULES := . api/gen
LINT_GOWORK ?= auto

BUF_GO_GEN_TEMPLATE := buf.go.gen.yaml

GOPATH := $(shell go env GOPATH)
GOVERSION := $(shell go version)

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date +%Y-%m-%dT%H:%M:%S)
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

LDFLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT) -X main.GitBranch=$(GIT_BRANCH)

RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
CYAN := \033[0;36m
RESET := \033[0m

# Docker compose (infrastructure only)
COMPOSE := docker compose
COMPOSE_FILES := -f docker-compose.yaml
INFRA_SERVICES := consul db redis openfga otel-collector jaeger loki prometheus traefik kafka clickhouse

# ============================================================================
# MAIN TARGETS
# ============================================================================

.PHONY: help env init plugin cli dep vendor tidy test cover vet lint lint.go lint.proto buf-update
.PHONY: api api-go gen clean
.PHONY: compose.up compose.stop compose.down compose.reset compose.ps compose.logs
.PHONY: ci.lint

env:
	@echo "CURRENT_DIR: $(CURRENT_DIR)"
	@echo "ROOT_DIR: $(ROOT_DIR)"
	@echo "VERSION: $(VERSION)"
	@echo "GOVERSION: $(GOVERSION)"

init: plugin cli
	@echo "$(GREEN)✓ Development environment initialized$(RESET)"

plugin:
	@echo "$(CYAN)Installing protoc plugins...$(RESET)"
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v2@latest
	@go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	@go install github.com/envoyproxy/protoc-gen-validate@latest
	@go install ./cmd/protoc-gen-servora-authz
	@go install ./cmd/protoc-gen-servora-audit
	@go install ./cmd/protoc-gen-servora-mapper
	@echo "$(GREEN)✓ Protoc plugins installed$(RESET)"

cli:
	@echo "$(CYAN)Installing CLI tools...$(RESET)"
	@go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	@go install github.com/google/gnostic@latest
	@go install github.com/bufbuild/buf/cmd/buf@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/google/wire/cmd/wire@latest
	@go install entgo.io/ent/cmd/ent@latest
	@go install ./cmd/svr
	@echo "$(GREEN)✓ CLI tools installed$(RESET)"

dep:
	@$(foreach mod,$(GO_WORKSPACE_MODULES),echo "  $(mod)" && (cd $(ROOT_DIR)$(mod) && go mod download) && ) true

vendor:
	@go mod vendor

tidy:
	@echo "$(CYAN)Tidying Go modules...$(RESET)"
	@$(foreach mod,$(GO_WORKSPACE_MODULES),echo "  $(mod)" && (cd $(ROOT_DIR)$(mod) && go mod tidy) && ) true
	@go work sync
	@echo "$(GREEN)✓ All modules tidied and workspace synced$(RESET)"

test:
	@$(foreach mod,$(GO_WORKSPACE_MODULES),echo "$(CYAN)Testing $(mod)...$(RESET)" && (cd $(ROOT_DIR)$(mod) && go test ./...) && ) true

cover:
	@$(foreach mod,$(GO_WORKSPACE_MODULES),(cd $(ROOT_DIR)$(mod) && go test -v ./... -coverprofile=coverage.out) && ) true

vet:
	@$(foreach mod,$(GO_WORKSPACE_MODULES),(cd $(ROOT_DIR)$(mod) && go vet ./...) && ) true

lint: lint.go
	@echo "$(GREEN)✓ lint complete$(RESET)"

lint.go:
	@$(foreach mod,$(GO_WORKSPACE_MODULES),echo "$(CYAN)Linting Go ($(mod), GOWORK=$(LINT_GOWORK))...$(RESET)" && (cd $(ROOT_DIR)$(mod) && GOWORK=$(LINT_GOWORK) golangci-lint run) && ) true
	@echo "$(GREEN)✓ Go lint complete$(RESET)"

# CI-equivalent lint path: reuse lint.go with GOWORK disabled, then lint proto.
ci.lint: LINT_GOWORK=off
ci.lint: lint.go lint.proto
	@echo "$(GREEN)✓ CI lint checks passed$(RESET)"

gen: api
	@echo "$(GREEN)✓ All code generated$(RESET)"

api: api-go
	@echo "$(GREEN)✓ Protobuf code generated$(RESET)"

api-go:
	@echo "$(CYAN)Generating protobuf Go code via $(BUF_GO_GEN_TEMPLATE)...$(RESET)"
	@buf generate --template $(BUF_GO_GEN_TEMPLATE)

lint.proto:
	@echo "$(CYAN)Linting protobuf files...$(RESET)"
	@buf lint
	@echo "$(GREEN)✓ Proto lint complete$(RESET)"

buf-update:
	@echo "$(CYAN)Updating buf dependencies...$(RESET)"
	@buf dep update
	@echo "$(GREEN)✓ Buf dependencies updated$(RESET)"

# Tag root module.
# Usage: make tag TAG=v0.2.0
tag:
ifndef TAG
	$(error TAG is required. Usage: make tag TAG=v0.2.0)
endif
	@echo "$(CYAN)Tagging $(TAG)...$(RESET)"
	@git tag $(TAG)
	@echo "$(GREEN)✓ Tagged: $(TAG)$(RESET)"
	@echo "  Run 'git push --tags' to push"

# Tag api/gen sub-module when proto/gen changes require it.
# Usage: make tag.api TAG=v0.2.0
tag.api:
ifndef TAG
	$(error TAG is required. Usage: make tag.api TAG=v0.2.0)
endif
	@echo "$(CYAN)Tagging api/gen/$(TAG)...$(RESET)"
	@git tag api/gen/$(TAG)
	@echo "$(GREEN)✓ Tagged: api/gen/$(TAG)$(RESET)"
	@echo "  Run 'git push --tags' to push"

# Push proto to BSR, auto-labeling with current Git tag if available
buf-push:
	@echo "$(CYAN)Pushing proto to BSR...$(RESET)"
	@GIT_TAG=$$(git tag --points-at HEAD 2>/dev/null | grep -E '^v[0-9]' | head -1); \
	if [ -n "$$GIT_TAG" ]; then \
		echo "  Using Git tag as BSR label: $$GIT_TAG"; \
		buf push --exclude-unnamed --label "$$GIT_TAG"; \
	else \
		echo "  $(YELLOW)No Git version tag on HEAD, pushing without label$(RESET)"; \
		buf push --exclude-unnamed; \
	fi
	@echo "$(GREEN)✓ Proto pushed to BSR$(RESET)"

# ============================================================================
# INFRASTRUCTURE COMPOSE TARGETS
# ============================================================================

compose.up:
	@echo "$(CYAN)Compose infra up: $(INFRA_SERVICES)$(RESET)"
	@$(COMPOSE) $(COMPOSE_FILES) up -d $(INFRA_SERVICES)
	@echo "$(GREEN)✓ Infrastructure services started$(RESET)"

compose.stop:
	@$(COMPOSE) $(COMPOSE_FILES) stop $(INFRA_SERVICES)

compose.down:
	@$(COMPOSE) $(COMPOSE_FILES) down --remove-orphans

compose.reset:
	@$(COMPOSE) $(COMPOSE_FILES) down --remove-orphans --volumes

compose.ps:
	@$(COMPOSE) $(COMPOSE_FILES) ps $(INFRA_SERVICES)

compose.logs:
	@$(COMPOSE) $(COMPOSE_FILES) logs -f $(INFRA_SERVICES)

# ============================================================================
# CLEANUP TARGETS
# ============================================================================

clean:
	@echo "$(CYAN)Cleaning build artifacts...$(RESET)"
	@rm -rf api/gen/go
	@echo "$(GREEN)✓ Clean complete$(RESET)"

help:
	@echo ""
	@echo "$(CYAN)servora Framework$(RESET)"
	@echo "$(CYAN)=================$(RESET)"
	@echo ""
	@echo "Usage:"
	@echo " make [target]"
	@echo ""
	@echo "Targets:"
	@awk '/^[a-zA-Z\-_0-9\.]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "  $(GREEN)%-20s$(RESET) %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
