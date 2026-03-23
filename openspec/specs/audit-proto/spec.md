# Spec: audit-proto

## Purpose

Defines requirements for the `audit-proto` capability.

## Requirements

### Requirement: Audit annotations proto defines RPC-level audit rules

`api/protos/servora/audit/v1/annotations.proto` SHALL define:
- `AuditRule` message with fields:
  - `bool enabled = 1` — whether to produce audit events (default true)
  - `AuditEventType event_type = 2` — event type enum
  - `ResourceMutationType mutation_type = 3` — CRUD semantics enum (replaces former `string operation`)
  - `string target_type = 4` — resource type string (e.g. "user", "project")
  - `string target_id_field = 5` — proto field path for target ID extraction (e.g. "req.id", "resp.id")
  - `bool record_on_error = 6` — emit event even on handler failure
- A method option `google.protobuf.MethodOptions` extension `audit_rule` (field number 50000) of type `AuditRule`

#### Scenario: Annotation proto compiles

- **WHEN** `make api` is run
- **THEN** `annotations.pb.go` SHALL be generated and the `audit_rule` extension SHALL be importable in Go

#### Scenario: Annotation can be applied to RPC with mutation_type

- **WHEN** a service proto imports `servora/audit/v1/annotations.proto` and annotates an RPC with `option (servora.audit.v1.audit_rule) = { event_type: AUDIT_EVENT_TYPE_RESOURCE_MUTATION, mutation_type: RESOURCE_MUTATION_TYPE_CREATE, target_type: "user", target_id_field: "resp.id" };`
- **THEN** the proto SHALL compile without errors

#### Scenario: mutation_type uses existing ResourceMutationType enum

- **WHEN** `annotations.proto` is inspected
- **THEN** the `mutation_type` field SHALL use the `ResourceMutationType` enum defined in `audit.proto` (values: UNSPECIFIED, CREATE, UPDATE, DELETE)
