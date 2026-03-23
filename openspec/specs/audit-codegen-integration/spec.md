# Spec: audit-codegen-integration

## Purpose

Defines requirements for the `audit-codegen-integration` capability.

## Requirements

### Requirement: buf.audit.gen.yaml configures audit code generation

A `buf.audit.gen.yaml` file SHALL exist at the repository root and configure the `protoc-gen-servora-audit` plugin with:
- `out: api/gen/go`
- `opt: paths=source_relative`

#### Scenario: buf generate with audit template

- **WHEN** `buf generate --template buf.audit.gen.yaml` is run
- **THEN** `audit_rules.gen.go` files SHALL be generated under `api/gen/go/` for any proto package with audit annotations

### Requirement: make api includes audit generation

The root `Makefile`'s `api` target SHALL invoke `buf generate --template buf.audit.gen.yaml` as part of its generation pipeline, alongside existing `buf.go.gen.yaml` and `buf.authz.gen.yaml`.

#### Scenario: make api generates all three

- **WHEN** `make api` is run
- **THEN** Go proto code, authz rules, and audit rules SHALL all be generated in sequence without errors

### Requirement: sayhello proto annotated with audit_rule

`app/sayhello/service/api/protos/servora/sayhello/service/v1/sayhello.proto` SHALL import `servora/audit/v1/annotations.proto` and annotate the `Hello` RPC with an `audit_rule` option specifying `enabled: true`, `event_type: AUDIT_EVENT_TYPE_RESOURCE_MUTATION`, `mutation_type: RESOURCE_MUTATION_TYPE_CREATE`, `target_type: "greeting"`, `record_on_error: true`.

#### Scenario: sayhello proto compiles with annotation

- **WHEN** `make api` is run after adding the annotation
- **THEN** the proto SHALL compile without errors and `audit_rules.gen.go` SHALL be generated in the sayhello Go package

### Requirement: sayhello grpc.go uses generated rules

After code generation, `app/sayhello/service/internal/server/grpc.go` SHALL replace the hand-written `audit.WithRules(map[string]audit.Rule{...})` with a reference to `sayhellopb.AuditRules()`, eliminating manual rule maintenance.

#### Scenario: Generated rules produce equivalent behavior

- **WHEN** the sayhello service starts with generated `AuditRules()`
- **AND** a `Hello` RPC is called
- **THEN** an audit event SHALL be emitted with `EventType=resource.mutation`, `TargetType="greeting"`, and `RecordOnError=true`, matching the behavior of the previous hand-written rule

### Requirement: E2E audit pipeline with generated rules

The full audit pipeline (sayhello → Kafka → audit service → ClickHouse → query API) SHALL work with generated rules, producing the same audit events as the previous hand-written configuration.

#### Scenario: E2E audit event visible via query API

- **WHEN** `make compose.dev` is running
- **AND** a `Hello` RPC is called on the sayhello service
- **THEN** the audit event SHALL appear in `GET /v1/audit/events` within 30 seconds
