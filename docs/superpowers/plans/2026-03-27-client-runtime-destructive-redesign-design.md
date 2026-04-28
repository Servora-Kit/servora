# Client Runtime Destructive Redesign (Server/Client Symmetric) - Design

**Date:** 2026-03-27
**Status:** Approved in brainstorming session
**Scope:** `servora` framework core (`transport/runtime`, `transport/server`, `transport/client`) + downstream compile-impact assessment

---

## 1. Background

Current `transport/server` and `transport/client` are both plugin-oriented, but client semantics still leak RPC-only assumptions:

- API shape is `ConnType + serviceName` instead of protocol-neutral dial input.
- gRPC client hard-codes governance middleware in plugin implementation.
- HTTP client implementation is not aligned with the same middleware-first governance path.
- Naming and docs imply enum-like protocol restriction while runtime is string-key plugin-based.

At the same time, user requirements are explicit:

- Keep package concepts as `server` and `client` (do not rename to inbound/outbound).
- Allow breaking changes without compatibility shims.
- Keep plugin architecture first-class.
- Governance should be middleware-first, matching existing server practice in IAM.

---

## 2. Design Decisions (Final)

### 2.1 Keep names `server` / `client`

No rename to `inbound` / `outbound` in this change.

### 2.2 Runtime is the single contract source

All plugin contracts live in `transport/runtime/contracts.go`.
`transport/client` must not redefine duplicate contract types.

### 2.3 `builtin` semantics are adjusted to `default plugins`

`grpc/http` are not privileged core internals; they are official default plugins living in the framework repo and registered by defaults.

### 2.4 Governance contract is middleware-first

For both server/client, governance is primarily represented as `github.com/go-kratos/kratos/v2/middleware` chains.

- trace/authn/authz/retry/circuit/logging/recovery => middleware domain
- metrics for request governance => middleware domain
- server-only exposure endpoints (e.g. `/metrics`, `/healthz`, swagger) are not client governance concerns

### 2.5 Third-party plugins can participate in governance

Governance is not limited to grpc/http by policy. Officially guaranteed support is grpc/http; third-party plugins can opt in by implementing required mapping points.

### 2.6 Pooling responsibility split

- Connection pool mechanics: plugin-owned (protocol-specific)
- Pool policy surface and lifecycle orchestration: framework contract-owned

---

## 3. Target Architecture

### 3.1 Symmetric transport layout

- `transport/runtime`: contracts, registry, graph
- `transport/server`: server builders and plugins
- `transport/client`: client manager/builders and plugins
- `transport/runtime/defaults` (renamed from builtin): default plugin registration

### 3.2 Client flow model

Replace current `CreateConn(ctx, connType, serviceName)` with protocol-neutral dial input:

- `ClientDialInput.Protocol`
- `ClientDialInput.Target`
- `ClientDialInput.ExtraValues`

Client manager resolves plugin by protocol and delegates dial behavior to plugin factory.

### 3.3 Plugin model

- Server plugins keep `Type() + Build(...)`
- Client plugins keep `Type() + Build(...)`, but factory API becomes dial-input based (not serviceName-only)

---

## 4. Governance Model

### 4.1 Server

Keep existing middleware chain model (`transport/server/middleware/chain.go`) as baseline governance path.

### 4.2 Client

Introduce first-class client middleware chain builder (`transport/client/middleware/chain.go`) and pass resulting chain through runtime client build input.

### 4.3 Plugin responsibility

Plugins must consume injected middleware chain when transport SDK supports it.
If a protocol cannot support middleware semantics directly, plugin must provide explicit adapter mapping or capability declaration.

---

## 5. Breaking Scope (No Compatibility Layer)

### 5.1 Remove old client call API

- Remove `ConnType` constants as public protocol control surface
- Remove `GetConnValue(ctx, client, connType, serviceName)` shape
- Remove `CreateConn(ctx, connType, serviceName)` from client interface

### 5.2 Introduce dial-input API

- New `Dial(ctx, ClientDialInput)` style APIs in runtime/client path

### 5.3 Rewrite grpc/http client plugin behavior

- gRPC: stop hard-coding middleware policy in plugin internals
- HTTP: align to same client governance injection model

---

## 6. Final File-Level Change Direction

This design is implemented by the approved destructive checklist (section 4 from brainstorming):

- Rewrite `transport/runtime/contracts.go`, graph and tests
- Replace `transport/client` public API and internals with dial-input model
- Add client middleware chain builder
- Rewrite `transport/client/grpc` and `transport/client/http` to consume framework-injected governance middleware
- Rename `transport/runtime/builtin` to `transport/runtime/defaults`
- Update all docs and downstream callers (`servora-example`, `servora-iam`)

---

## 7. Non-Goals

- No attempt to preserve old client API compatibility.
- No broad protocol governance parity requirement for every third-party plugin in this phase.
- No server package rename.

---

## 8. Acceptance Criteria

1. `server/client` remain plugin-driven and symmetric at runtime contract level.
2. Client governance is middleware-first and framework-injected.
3. grpc/http default plugins comply with the new client dial contract.
4. Old conn-type/service-name API is fully removed from framework and examples.
5. `go test ./...` passes in `servora` and impacted downstream repos after migration updates.

