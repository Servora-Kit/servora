# ============================================================================
# Makefile for servora Project
# ============================================================================
# Based on go-wind-admin project structure
# ============================================================================

ifeq ($(OS),Windows_NT)
    IS_WINDOWS := 1
endif

# load environment variables from .env file if it exists
ifneq (,$(wildcard .env))
    include .env
    export
endif

# ============================================================================
# VARIABLES & CONFIGURATION
# ============================================================================

# Directories
CURRENT_DIR := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
ROOT_DIR    := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))
API_DIR     := api
PKG_DIR     := pkg

# Buf generation templates (fixed filenames; OpenAPI uses per-service files via app.mk)
BUF_GO_GEN_TEMPLATE := buf.go.gen.yaml
BUF_TS_GEN_TEMPLATE := buf.typescript.gen.yaml

# Find all service Makefiles in app directory
SRCS_MK := $(foreach dir, app, $(wildcard $(dir)/*/*/Makefile))
SERVICE_DIRS := $(dir $(realpath $(SRCS_MK)))

# Go environment
GOPATH := $(shell go env GOPATH)
GOVERSION := $(shell go version)

# Build information
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date +%Y-%m-%dT%H:%M:%S)
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

# LDFLAGS
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT) -X main.GitBranch=$(GIT_BRANCH)

# Output colors
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
CYAN := \033[0;36m
RESET := \033[0m

# Docker compose
COMPOSE := docker compose
COMPOSE_FILES := -f docker-compose.yaml
COMPOSE_DEV_FILES := -f docker-compose.yaml -f docker-compose.dev.yaml
DOCKER_BAKE_FILE := docker-bake.hcl
BAKE_TARGETS ?= default
MICROSERVICES := servora sayhello
INFRA_SERVICES := consul db redis otel-collector jaeger loki prometheus grafana
COMPOSE_STACK_SERVICES := $(INFRA_SERVICES) $(MICROSERVICES)
COMPOSE_STACK_DOWN := $(COMPOSE) $(COMPOSE_DEV_FILES) down --remove-orphans
COMPOSE_STACK_RESET := $(COMPOSE) $(COMPOSE_DEV_FILES) down --remove-orphans --volumes

define run-in-service-dirs
	@$(foreach dir,$(SERVICE_DIRS),cd $(dir) && $(MAKE) $(1);)
endef

# ============================================================================
# MAIN TARGETS
# ============================================================================

.PHONY: help env init plugin cli dep vendor test cover vet lint.go lint.proto buf-update
.PHONY: wire ent gen api api-go api-ts openapi build all clean
.PHONY: compose.build compose.up compose.rebuild compose.stop compose.down compose.reset compose.ps compose.logs
.PHONY: compose.dev compose.dev.build compose.dev.up compose.dev.restart compose.dev.ps compose.dev.stop compose.dev.down compose.dev.reset compose.dev.logs

# show environment variables
env:
	@echo "CURRENT_DIR: $(CURRENT_DIR)"
	@echo "ROOT_DIR: $(ROOT_DIR)"
	@echo "SRCS_MK: $(SRCS_MK)"
	@echo "VERSION: $(VERSION)"
	@echo "GOVERSION: $(GOVERSION)"

# initialize develop environment
init: plugin cli
	@echo "$(GREEN)✓ Development environment initialized$(RESET)"

# install protoc plugins
plugin:
	@echo "$(CYAN)Installing protoc plugins...$(RESET)"
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	@go install github.com/go-kratos/protoc-gen-typescript-http@latest
	@go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v2@latest
	@go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	@go install github.com/envoyproxy/protoc-gen-validate@latest
	@echo "$(GREEN)✓ Protoc plugins installed$(RESET)"

# install cli tools
cli:
	@echo "$(CYAN)Installing CLI tools...$(RESET)"
	@go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	@go install github.com/google/gnostic@latest
	@go install github.com/bufbuild/buf/cmd/buf@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/google/wire/cmd/wire@latest
	@go install entgo.io/ent/cmd/ent@latest
	@echo "$(GREEN)✓ CLI tools installed$(RESET)"

# download dependencies of module
dep:
	@go mod download

# create vendor
vendor:
	@go mod vendor

# run tests
test:
	@go test ./...

# run coverage tests
cover:
	@go test -v ./... -coverprofile=coverage.out

# run static analysis
vet:
	@go vet ./...

# run golang lint
lint.go:
	@golangci-lint run

# generate wire code for all services
wire:
	@echo "$(CYAN)Generating wire code for all services...$(RESET)"
	$(call run-in-service-dirs,wire)
	@echo "$(GREEN)✓ Wire code generated$(RESET)"

# generate ent code for services that define data/generate.go
ent:
	@echo "$(CYAN)Generating ent code for all services...$(RESET)"
	$(call run-in-service-dirs,gen.ent)
	@echo "$(GREEN)✓ Ent code generated$(RESET)"

# generate all code
gen: api openapi wire ent
	@echo "$(GREEN)✓ All code generated$(RESET)"

# generate protobuf api code (go + ts)
api: api-go
	@echo "$(GREEN)✓ Protobuf code generated $(RESET)"

# generate protobuf api go code
api-go:
	@echo "$(CYAN)Generating protobuf Go code via $(BUF_GO_GEN_TEMPLATE)...$(RESET)"
	@buf generate --template $(BUF_GO_GEN_TEMPLATE)

# generate protobuf api typescript code for web
api-ts:
	@echo "$(CYAN)Generating protobuf TypeScript code via $(BUF_TS_GEN_TEMPLATE)...$(RESET)"
	@buf generate --template $(BUF_TS_GEN_TEMPLATE)

# generate protobuf api OpenAPI v3 docs for all services
openapi:
	@echo "$(CYAN)Generating OpenAPI documentation for all services...$(RESET)"
	$(call run-in-service-dirs,openapi)
	@echo "$(GREEN)✓ OpenAPI documentation generated$(RESET)"

# lint protobuf files
lint.proto:
	@echo "$(CYAN)Linting protobuf files...$(RESET)"
	@buf lint
	@echo "$(GREEN)✓ Proto lint complete$(RESET)"

# update buf dependencies
buf-update:
	@echo "$(CYAN)Updating buf dependencies...$(RESET)"
	@buf dep update
	@echo "$(GREEN)✓ Buf dependencies updated$(RESET)"

# build all service applications
build: gen
	@echo "$(CYAN)Building all services...$(RESET)"
	$(call run-in-service-dirs,_build)
	@echo "$(GREEN)✓ All services built$(RESET)"

# generate & build all service applications
all:
	@echo "$(CYAN)Generating and building all services...$(RESET)"
	$(call run-in-service-dirs,app)
	@echo "$(GREEN)✓ All services generated and built$(RESET)"

# build production images for microservices
compose.build:
	@echo "$(CYAN)Build production images via Bake: $(BAKE_TARGETS) (version: $(VERSION))$(RESET)"
	@VERSION=$(VERSION) docker buildx bake --file $(DOCKER_BAKE_FILE) $(BAKE_TARGETS)
	@echo "$(GREEN)✓ Production images built$(RESET)"

# start infrastructure compose stack
compose.up:
	@echo "$(CYAN)Compose infra up: $(INFRA_SERVICES)$(RESET)"
	@$(COMPOSE) $(COMPOSE_FILES) up -d $(INFRA_SERVICES)
	@echo "$(GREEN)✓ Infrastructure services started$(RESET)"

# rebuild production images and ensure infrastructure is running
compose.rebuild:
	@$(MAKE) compose.build
	@$(MAKE) compose.up
	@echo "$(GREEN)✓ Production images rebuilt and infrastructure started$(RESET)"

# stop infrastructure compose stack
compose.stop:
	@$(COMPOSE) $(COMPOSE_FILES) stop $(INFRA_SERVICES)

# remove local compose stack containers/networks
compose.down:
	@$(COMPOSE_STACK_DOWN)

# remove local compose stack containers/networks/volumes
compose.reset:
	@$(COMPOSE_STACK_RESET)

# show infrastructure compose stack status
compose.ps:
	@$(COMPOSE) $(COMPOSE_FILES) ps $(INFRA_SERVICES)

# tail logs for infrastructure compose stack
compose.logs:
	@$(COMPOSE) $(COMPOSE_FILES) logs -f $(INFRA_SERVICES)

# build Air-based development images for microservices
compose.dev.build:
	@echo "$(CYAN)Compose dev build: $(MICROSERVICES)$(RESET)"
	@$(COMPOSE) $(COMPOSE_DEV_FILES) build $(MICROSERVICES)
	@echo "$(GREEN)✓ Compose dev images built$(RESET)"

# start full development compose stack (infra + Air microservices) and tail logs
compose.dev:
	@echo "$(CYAN)Compose dev start: $(COMPOSE_STACK_SERVICES)$(RESET)"
	@$(COMPOSE) $(COMPOSE_DEV_FILES) up -d $(COMPOSE_STACK_SERVICES)
	@echo "$(GREEN)✓ Compose dev full stack started, tailing logs...$(RESET)"
	@$(COMPOSE) $(COMPOSE_DEV_FILES) logs -f $(COMPOSE_STACK_SERVICES)

# start Air-based development stack in background
compose.dev.up:
	@echo "$(CYAN)Compose dev up: $(COMPOSE_STACK_SERVICES)$(RESET)"
	@$(COMPOSE) $(COMPOSE_DEV_FILES) up -d $(COMPOSE_STACK_SERVICES)
	@echo "$(GREEN)✓ Compose dev stack started$(RESET)"

# restart Air-based development containers to force fresh startup build
compose.dev.restart:
	@echo "$(CYAN)Compose dev restart (Air): $(MICROSERVICES)$(RESET)"
	@$(COMPOSE) $(COMPOSE_DEV_FILES) restart $(MICROSERVICES)
	@echo "$(GREEN)✓ Compose dev services restarted$(RESET)"

# tail logs for Air-based development stack
compose.dev.logs:
	@$(COMPOSE) $(COMPOSE_DEV_FILES) logs -f $(MICROSERVICES)

# show Air-based development stack status
compose.dev.ps:
	@$(COMPOSE) $(COMPOSE_DEV_FILES) ps $(COMPOSE_STACK_SERVICES)

# stop Air-based development stack without removing containers
compose.dev.stop:
	@$(COMPOSE) $(COMPOSE_DEV_FILES) stop $(COMPOSE_STACK_SERVICES)

# remove Air-based development stack containers/networks
compose.dev.down:
	@$(COMPOSE_STACK_DOWN)

# remove Air-based development stack containers/networks/volumes
compose.dev.reset:
	@$(COMPOSE_STACK_RESET)

# clean build artifacts
clean:
	@echo "$(CYAN)Cleaning build artifacts...$(RESET)"
	@rm -rf api/gen/go
	$(call run-in-service-dirs,clean)
	@echo "$(GREEN)✓ Clean complete$(RESET)"

# show help
help:
	@echo ""
	@echo "$(CYAN)servora Development Environment$(RESET)"
	@echo "$(CYAN)=================================$(RESET)"
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
