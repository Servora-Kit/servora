# Authz Closure & Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the authz middleware loop (audit bridge), expose OpenFGA's BatchCheck/ListObjects through the `Authorizer` interface, and harden the middleware with timeout / fail-open / nested-IDField — while removing the always-`false` `CacheHit` field that misrepresents audit truth.

**Architecture:** Extend `security/authz.Authorizer` from a single `IsAuthorized` method to three methods (`Check` / `BatchCheck` / `ListAllowed`), all natively supported by OpenFGA SDK v0.7.5 and trivially implemented by the noop authorizer. Cache is preserved as opt-in via `WithRedisCache` but hidden from middleware (cache-hit metrics live in `infra/openfga`, not in audit detail). A new `authz.NewAuthzBridge(recorder)` helper closes the P0-2 loop by auto-forwarding every `DecisionDetail` to `Recorder.RecordAuthzDecision` — no business code needs to wire it manually.

**Tech Stack:** Go 1.22+ / Kratos v2 middleware / OpenFGA SDK v0.7.5 / `obs/audit` Recorder

**Breaking changes (in-tree only — no live business callers):**
- `authz.Authorizer.IsAuthorized` → renamed `Check`, plus two new required methods (`BatchCheck`, `ListAllowed`)
- `authz.DecisionDetail.CacheHit` field removed
- `audit.AuthzDetail.CacheHit` field removed
- `openfga.Authorizer.IsAuthorized` → `Check` (impl rename)
- `noop.Authorizer.IsAuthorized` → `Check` (impl rename)

---

## Pre-flight

- [ ] **Step 0: Verify clean working tree**

Run: `cd servora && git status --short`
Expected: empty output (clean tree). If dirty, stash or commit first.

- [ ] **Step 0b: Verify baseline tests pass**

Run: `cd servora && go test ./security/... ./obs/audit/... ./infra/openfga/... -race`
Expected: all PASS. This is the baseline we must preserve.

---

## File Structure

### Files to modify
- `servora/security/authz/authz.go` — interface + middleware logic
- `servora/security/authz/authz_test.go` — tests
- `servora/security/authz/AGENTS.md` — doc consistency
- `servora/security/authz/noop/noop.go` — noop impl gains `BatchCheck` / `ListAllowed`
- `servora/security/authz/openfga/openfga.go` — openfga impl gains `BatchCheck` / `ListAllowed`
- `servora/infra/openfga/check.go` — add `Client.BatchCheck`
- `servora/obs/audit/event.go` — remove `AuthzDetail.CacheHit`
- `servora/docs/TODO.md` — mark P0-2 done, add this plan to "已完成"

### Files to create
- `servora/security/authz/bridge.go` — `NewAuthzBridge(recorder)` helper (P0-2)
- `servora/security/authz/bridge_test.go` — bridge tests
- `servora/security/authz/doc.go` — package-level doc with contextual-tuples future note
- `servora/infra/openfga/check_batch_test.go` — BatchCheck input-shaping unit tests

### Out of scope (not done in this plan)
- Contextual tuples implementation (placeholder doc only — YAGNI until ABAC business need)
- SHADOW / LOG_ONLY AuthzMode (proto enum unchanged)
- Metrics emission (deferred per user instruction)
- Subject mapper (framework convention: `actor.Type` ↔ OpenFGA `type` aligned by design)
- Recovery wrapper (kratos `recovery.Recovery()` is already chain-outermost)
- Cache code deletion (kept as opt-in via `WithRedisCache`)

---

## Task 1: Remove `audit.AuthzDetail.CacheHit` field

**Why first:** `authz` middleware will stop populating this field; keeping it silently `false` perpetuates the bug. Audit semantics shouldn't expose engine-internal optimization (cache hit).

**Files:**
- Modify: `servora/obs/audit/event.go:77-84`

- [ ] **Step 1.1: Search for any in-tree caller setting `CacheHit:`**

Run: `cd servora && grep -rn "CacheHit:" --include="*.go" .`
Expected: zero matches (the field is defined but never written, which is exactly the bug).

If matches found: those callers must be removed in this same task.

- [ ] **Step 1.2: Remove the field**

Edit `servora/obs/audit/event.go`, current lines 77-84:

```go
// AuthzDetail carries authorization-decision detail.
type AuthzDetail struct {
    Relation    string
    ObjectType  string
    ObjectID    string
    Decision    AuthzDecision
    CacheHit    bool       // ← remove this line
    ErrorReason string
}
```

After:

```go
// AuthzDetail carries authorization-decision detail.
// Cache-hit metrics live in infra/openfga (engine-internal optimization),
// not in audit semantics.
type AuthzDetail struct {
    Relation    string
    ObjectType  string
    ObjectID    string
    Decision    AuthzDecision
    ErrorReason string
}
```

- [ ] **Step 1.3: Run audit tests**

Run: `cd servora && go test ./obs/audit/... -race`
Expected: PASS (no test depended on `CacheHit`).

- [ ] **Step 1.4: Run full build to find any external setter**

Run: `cd servora && go build ./...`
Expected: PASS. If build fails because some `*.go` uses `CacheHit:`, remove that usage.

- [ ] **Step 1.5: Commit**

```bash
cd servora
git add obs/audit/event.go
git commit -m "refactor(obs/audit): drop AuthzDetail.CacheHit (engine-internal, not audit semantics)"
```

---

## Task 2: Add `Client.BatchCheck` to `infra/openfga`

**Why now:** `Authorizer.BatchCheck` (Task 4) needs this primitive. Build it bottom-up.

**Files:**
- Modify: `servora/infra/openfga/check.go`
- Create: `servora/infra/openfga/check_batch_test.go`

- [ ] **Step 2.1: Write the failing test for input-shape correctness**

We can't easily mock the OpenFGA SDK client end-to-end without a server, so the test verifies *input shaping* via dedicated helpers we extract.

Create `servora/infra/openfga/check_batch_test.go`:

```go
package openfga

import (
	"errors"
	"testing"

	fgaclient "github.com/openfga/go-sdk/client"
)

func TestBuildBatchCheckItems_PreservesOrderAndMapping(t *testing.T) {
	reqs := []BatchCheckItem{
		{User: "user:alice", Relation: "viewer", Object: "doc:1"},
		{User: "user:bob", Relation: "editor", Object: "doc:2"},
		{User: "user:carol", Relation: "owner", Object: "doc:3"},
	}

	items := buildBatchCheckItems(reqs)

	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}
	for i, item := range items {
		if item.User != reqs[i].User {
			t.Errorf("item[%d].User = %q, want %q", i, item.User, reqs[i].User)
		}
		if item.Relation != reqs[i].Relation {
			t.Errorf("item[%d].Relation = %q, want %q", i, item.Relation, reqs[i].Relation)
		}
		if item.Object != reqs[i].Object {
			t.Errorf("item[%d].Object = %q, want %q", i, item.Object, reqs[i].Object)
		}
		// CorrelationId must be the index — used to map response back.
		wantCorr := correlationIDFromIndex(i)
		if item.CorrelationId != wantCorr {
			t.Errorf("item[%d].CorrelationId = %q, want %q", i, item.CorrelationId, wantCorr)
		}
	}

	// Ensure type assignment compiles (the helper actually emits ClientBatchCheckItem).
	var _ []fgaclient.ClientBatchCheckItem = items
}

func TestMapBatchCheckResults_BackToOrderedResults(t *testing.T) {
	allowed := map[string]bool{
		correlationIDFromIndex(0): true,
		correlationIDFromIndex(1): false,
		correlationIDFromIndex(2): true,
	}
	errs := map[string]error{
		correlationIDFromIndex(1): errors.New("backend boom"),
	}

	out := mapBatchCheckResults(3, allowed, errs)

	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3", len(out))
	}
	if !out[0].Allowed || out[1].Allowed || !out[2].Allowed {
		t.Errorf("allowed pattern = [%v %v %v], want [true false true]",
			out[0].Allowed, out[1].Allowed, out[2].Allowed)
	}
	if out[1].Err == nil {
		t.Errorf("out[1].Err = nil, want non-nil")
	}
	if out[0].Err != nil || out[2].Err != nil {
		t.Errorf("out[0].Err=%v out[2].Err=%v, both want nil", out[0].Err, out[2].Err)
	}
}

func TestBatchCheck_EmptyInput_ReturnsNilNil(t *testing.T) {
	c := &Client{}
	got, err := c.BatchCheck(nil, nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}
```

- [ ] **Step 2.2: Run the test (expect FAIL — types not defined)**

Run: `cd servora && go test ./infra/openfga/ -run "TestBuildBatchCheckItems|TestMapBatchCheckResults|TestBatchCheck_Empty" -v`
Expected: FAIL with `undefined: BatchCheckItem`, `undefined: buildBatchCheckItems`, etc.

- [ ] **Step 2.3: Implement `BatchCheck` and helpers**

Add to `servora/infra/openfga/check.go` (append, keep existing `Check`):

```go
// BatchCheckItem is one element in a BatchCheck request.
// It mirrors fgaclient.ClientBatchCheckItem but is part of our stable API.
type BatchCheckItem struct {
	User     string
	Relation string
	Object   string
}

// BatchCheckResult is the per-item outcome from BatchCheck.
// Order matches the input slice index.
type BatchCheckResult struct {
	Allowed bool
	Err     error
}

// BatchCheck runs N checks in one OpenFGA call. Output order matches input order.
// Returns a top-level error only if the whole call fails; per-item errors land
// in BatchCheckResult.Err for that item.
func (c *Client) BatchCheck(ctx context.Context, reqs []BatchCheckItem) ([]BatchCheckResult, error) {
	if len(reqs) == 0 {
		return nil, nil
	}

	items := buildBatchCheckItems(reqs)

	resp, err := c.sdk.BatchCheck(ctx).
		Body(fgaclient.ClientBatchCheckRequest{Checks: items}).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("openfga batch check: %w", err)
	}

	allowed := make(map[string]bool, len(reqs))
	errs := make(map[string]error)
	for corr, single := range resp.GetResult() {
		allowed[corr] = single.GetAllowed()
		if e := single.Error; e != nil {
			errs[corr] = fmt.Errorf("openfga batch item %s: %s", corr, e.GetMessage())
		}
	}

	return mapBatchCheckResults(len(reqs), allowed, errs), nil
}

func buildBatchCheckItems(reqs []BatchCheckItem) []fgaclient.ClientBatchCheckItem {
	items := make([]fgaclient.ClientBatchCheckItem, len(reqs))
	for i, r := range reqs {
		items[i] = fgaclient.ClientBatchCheckItem{
			User:          r.User,
			Relation:      r.Relation,
			Object:        r.Object,
			CorrelationId: correlationIDFromIndex(i),
		}
	}
	return items
}

func mapBatchCheckResults(n int, allowed map[string]bool, errs map[string]error) []BatchCheckResult {
	out := make([]BatchCheckResult, n)
	for i := 0; i < n; i++ {
		corr := correlationIDFromIndex(i)
		out[i] = BatchCheckResult{
			Allowed: allowed[corr],
			Err:     errs[corr],
		}
	}
	return out
}

func correlationIDFromIndex(i int) string {
	return strconv.Itoa(i)
}
```

Add `"strconv"` to the import block at top of file.

- [ ] **Step 2.4: Verify the tests now pass**

Run: `cd servora && go test ./infra/openfga/ -run "TestBuildBatchCheckItems|TestMapBatchCheckResults|TestBatchCheck_Empty" -v -race`
Expected: PASS.

- [ ] **Step 2.5: Run all openfga tests**

Run: `cd servora && go test ./infra/openfga/... -race`
Expected: PASS (no regression).

- [ ] **Step 2.6: Commit**

```bash
cd servora
git add infra/openfga/check.go infra/openfga/check_batch_test.go
git commit -m "feat(infra/openfga): add Client.BatchCheck with per-item correlation"
```

---

## Task 3: Drop `authz.DecisionDetail.CacheHit`

**Why before Task 4:** The interface change in Task 4 wants a clean `DecisionDetail`. Drop the unused-and-misleading field first as a pure deletion.

**Files:**
- Modify: `servora/security/authz/authz.go:50-60` (struct definition)
- Modify: `servora/security/authz/AGENTS.md`

- [ ] **Step 3.1: Search for any `CacheHit` reference inside servora/security/authz/**

Run: `cd servora && grep -rn "CacheHit" security/authz/`
Expected: 2 matches — definition (`authz.go:58`) and AGENTS.md mention (`AGENTS.md:55`).

- [ ] **Step 3.2: Remove the field from `DecisionDetail`**

Edit `servora/security/authz/authz.go`:

Find:
```go
// DecisionDetail describes the result of a single authorization check.
// It is passed to the DecisionLogger callback after every check.
type DecisionDetail struct {
	Operation  string
	Subject    string
	Relation   string
	ObjectType string
	ObjectID   string
	Allowed    bool
	CacheHit   bool
	Err        error
}
```

Replace with:
```go
// DecisionDetail describes the result of a single authorization check.
// It is passed to the DecisionLogger callback after every check.
//
// Cache-hit signals are intentionally absent — caching is an engine-internal
// optimization (see infra/openfga) and does not belong in audit semantics.
type DecisionDetail struct {
	Operation  string
	Subject    string
	Relation   string
	ObjectType string
	ObjectID   string
	Allowed    bool
	Err        error
}
```

- [ ] **Step 3.3: Update `AGENTS.md` mention**

Edit `servora/security/authz/AGENTS.md` line 55:

Find: `` - `DecisionDetail` 包含 `Operation`、`Subject`、`Relation`、`ObjectType`、`ObjectID`、`Allowed`、`CacheHit`、`Err` ``

Replace with: `` - `DecisionDetail` 包含 `Operation`、`Subject`、`Relation`、`ObjectType`、`ObjectID`、`Allowed`、`Err`（cache 命中不进审计语义，留在 `infra/openfga` 内部） ``

- [ ] **Step 3.4: Run authz tests**

Run: `cd servora && go test ./security/authz/... -race`
Expected: PASS.

- [ ] **Step 3.5: Commit**

```bash
cd servora
git add security/authz/authz.go security/authz/AGENTS.md
git commit -m "refactor(security/authz): drop DecisionDetail.CacheHit (cache is engine-internal)"
```

---

## Task 4: Extend `Authorizer` interface to three methods

**Why this shape:** Both ReBAC mainstream backends (OpenFGA, SpiceDB) natively support all three. Other paradigms (Cedar/Rego) would require a full authz rewrite anyway, so capability sub-interfaces buy nothing here. Engine method names match OpenFGA SDK for direct mapping.

**Files:**
- Modify: `servora/security/authz/authz.go` (interface + middleware callsite)
- Modify: `servora/security/authz/authz_test.go` (`fakeAuthorizer` impl)
- Modify: `servora/security/authz/noop/noop.go`
- Modify: `servora/security/authz/openfga/openfga.go`

- [ ] **Step 4.1: Write failing test for new interface methods**

Append to `servora/security/authz/authz_test.go`:

```go
// TestFakeAuthorizer_ImplementsBatchCheck ensures the test fake covers BatchCheck.
func TestFakeAuthorizer_ImplementsBatchCheck(t *testing.T) {
	a := &fakeAuthorizer{allowed: true}
	results, err := a.BatchCheck(context.Background(), []CheckRequest{
		{Subject: "user:alice", Relation: "viewer", ObjectType: "doc", ObjectID: "1"},
		{Subject: "user:alice", Relation: "viewer", ObjectType: "doc", ObjectID: "2"},
	})
	if err != nil {
		t.Fatalf("BatchCheck err = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if !results[0].Allowed || !results[1].Allowed {
		t.Errorf("results = %+v, want all allowed", results)
	}
}

// TestFakeAuthorizer_ImplementsListAllowed ensures the test fake covers ListAllowed.
func TestFakeAuthorizer_ImplementsListAllowed(t *testing.T) {
	a := &fakeAuthorizer{listAllowedIDs: []string{"doc:1", "doc:5"}}
	ids, err := a.ListAllowed(context.Background(), "user:alice", "viewer", "doc")
	if err != nil {
		t.Fatalf("ListAllowed err = %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("len(ids) = %d, want 2", len(ids))
	}
}
```

- [ ] **Step 4.2: Run the test (expect compile failure)**

Run: `cd servora && go test ./security/authz/ -run TestFakeAuthorizer -v`
Expected: FAIL — `undefined: CheckRequest`, `a.BatchCheck undefined`, etc.

- [ ] **Step 4.3: Update interface**

Edit `servora/security/authz/authz.go`.

Find:
```go
// Authorizer is the interface for checking authorization.
// Implementations are responsible for performing the actual permission check,
// including any caching or backend communication.
type Authorizer interface {
	IsAuthorized(ctx context.Context, subject, relation, objectType, objectID string) (allowed bool, err error)
}
```

Replace with:
```go
// CheckRequest is one item in a BatchCheck call.
type CheckRequest struct {
	Subject    string
	Relation   string
	ObjectType string
	ObjectID   string
}

// CheckResult is the per-item outcome of BatchCheck.
// Order matches the input []CheckRequest index.
type CheckResult struct {
	Allowed bool
	Err     error
}

// Authorizer is the interface for relationship-based authorization decisions.
// All three methods are required: implementations targeting non-ReBAC backends
// (e.g. pure Cedar/Rego) would need a different abstraction entirely, so we
// commit to the ReBAC shape rather than a sub-interface fan-out.
//
// Method names match OpenFGA SDK semantics for direct mapping; SpiceDB
// (LookupResources / BulkCheck) maps cleanly as well.
type Authorizer interface {
	// Check returns whether subject has relation on objectType:objectID.
	Check(ctx context.Context, subject, relation, objectType, objectID string) (allowed bool, err error)

	// BatchCheck runs N checks in one round-trip; output order matches input.
	// Implementations may internally chunk if the backend has per-call limits
	// (OpenFGA caps at 50 per request).
	BatchCheck(ctx context.Context, reqs []CheckRequest) ([]CheckResult, error)

	// ListAllowed returns IDs of objects (of objectType) the subject has the
	// given relation to. The returned strings are bare IDs without "type:" prefix.
	// Useful for "list" endpoints — caller fetches by `WHERE id IN (...)`.
	ListAllowed(ctx context.Context, subject, relation, objectType string) ([]string, error)
}
```

- [ ] **Step 4.4: Update middleware callsite in `authz.go`**

In `Server()` (around line 188), find:
```go
allowed, err := authorizer.IsAuthorized(ctx, principal, relation, objectType, objectID)
```

Replace with:
```go
allowed, err := authorizer.Check(ctx, principal, relation, objectType, objectID)
```

- [ ] **Step 4.5: Update `fakeAuthorizer` in `authz_test.go`**

Find the `fakeAuthorizer` struct in `authz_test.go:45-52`:
```go
type fakeAuthorizer struct {
	allowed bool
	err     error
}

func (f *fakeAuthorizer) IsAuthorized(_ context.Context, _, _, _, _ string) (bool, error) {
	return f.allowed, f.err
}
```

Replace with:
```go
type fakeAuthorizer struct {
	allowed        bool
	err            error
	listAllowedIDs []string
}

func (f *fakeAuthorizer) Check(_ context.Context, _, _, _, _ string) (bool, error) {
	return f.allowed, f.err
}

func (f *fakeAuthorizer) BatchCheck(_ context.Context, reqs []CheckRequest) ([]CheckResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([]CheckResult, len(reqs))
	for i := range reqs {
		out[i] = CheckResult{Allowed: f.allowed}
	}
	return out, nil
}

func (f *fakeAuthorizer) ListAllowed(_ context.Context, _, _, _ string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.listAllowedIDs, nil
}
```

- [ ] **Step 4.6: Update `noop` impl**

Edit `servora/security/authz/noop/noop.go`. Final file should be exactly:

```go
// Package noop provides a no-op Authorizer that always permits all requests.
// Useful for testing or services that do not require authorization enforcement.
package noop

import (
	"context"

	"github.com/Servora-Kit/servora/security/authz"
)

// Ensure *Authorizer implements authz.Authorizer at compile time.
var _ authz.Authorizer = (*Authorizer)(nil)

// Authorizer is a no-op implementation that always returns allowed=true.
type Authorizer struct{}

// New returns a NoopAuthorizer that always permits requests.
func New() authz.Authorizer { return &Authorizer{} }

// Check always returns (true, nil).
func (a *Authorizer) Check(_ context.Context, _, _, _, _ string) (bool, error) {
	return true, nil
}

// BatchCheck returns all-allowed results matching the input length.
func (a *Authorizer) BatchCheck(_ context.Context, reqs []authz.CheckRequest) ([]authz.CheckResult, error) {
	out := make([]authz.CheckResult, len(reqs))
	for i := range reqs {
		out[i] = authz.CheckResult{Allowed: true}
	}
	return out, nil
}

// ListAllowed returns nil — the noop authorizer has no resource model.
// Callers needing real listing must use a real backend.
func (a *Authorizer) ListAllowed(_ context.Context, _, _, _ string) ([]string, error) {
	return nil, nil
}
```

- [ ] **Step 4.7: Update `openfga` impl**

Edit `servora/security/authz/openfga/openfga.go`. Replace the existing `IsAuthorized` method with three new methods. Final method block:

```go
// Check uses CachedCheck (which falls back to plain Check when redis is nil).
// Cache-hit signals stay inside this package — they are not exposed via DecisionDetail.
func (a *Authorizer) Check(ctx context.Context, subject, relation, objectType, objectID string) (bool, error) {
	allowed, _, err := a.client.CachedCheck(ctx, a.cfg.redis, a.cfg.cacheTTL,
		subject, relation, objectType, objectID)
	return allowed, err
}

// BatchCheck delegates to *openfga.Client.BatchCheck.
// Cache is intentionally NOT consulted for batch checks — N Redis lookups would
// negate the batching win. Callers needing cached batch behavior should issue
// N Check calls instead.
func (a *Authorizer) BatchCheck(ctx context.Context, reqs []authz.CheckRequest) ([]authz.CheckResult, error) {
	if len(reqs) == 0 {
		return nil, nil
	}

	items := make([]pkgfga.BatchCheckItem, len(reqs))
	for i, r := range reqs {
		items[i] = pkgfga.BatchCheckItem{
			User:     r.Subject,
			Relation: r.Relation,
			Object:   r.ObjectType + ":" + r.ObjectID,
		}
	}

	results, err := a.client.BatchCheck(ctx, items)
	if err != nil {
		return nil, err
	}

	out := make([]authz.CheckResult, len(results))
	for i, r := range results {
		out[i] = authz.CheckResult{Allowed: r.Allowed, Err: r.Err}
	}
	return out, nil
}

// ListAllowed delegates to *openfga.Client.CachedListObjects (cache opt-in).
func (a *Authorizer) ListAllowed(ctx context.Context, subject, relation, objectType string) ([]string, error) {
	return a.client.CachedListObjects(ctx, a.cfg.redis, pkgfga.DefaultListCacheTTL,
		subject, relation, objectType)
}
```

(Delete the existing `IsAuthorized` method from this file.)

- [ ] **Step 4.8: Run all authz tests**

Run: `cd servora && go test ./security/authz/... -race`
Expected: PASS — including the new `TestFakeAuthorizer_ImplementsBatchCheck` and `TestFakeAuthorizer_ImplementsListAllowed`.

- [ ] **Step 4.9: Run full build to catch any external caller**

Run: `cd servora && go build ./...`
Expected: PASS. If anywhere else in `servora/` references `IsAuthorized`, fix it (rename to `Check`).

- [ ] **Step 4.10: Commit**

```bash
cd servora
git add security/authz/authz.go security/authz/authz_test.go \
        security/authz/noop/noop.go security/authz/openfga/openfga.go
git commit -m "feat(security/authz): expand Authorizer to Check/BatchCheck/ListAllowed

Breaking: IsAuthorized renamed to Check. Two new required methods
(BatchCheck, ListAllowed) align with OpenFGA SDK and SpiceDB primitives.
No in-tree callers exist (servora-iam deprecated, servora-platform on v0.1.2)."
```

---

## Task 5: Add `WithCheckTimeout` option

**Why:** OpenFGA slow queries can drag business-RPC SLA. Bound the check to a configurable max latency.

**Files:**
- Modify: `servora/security/authz/authz.go`
- Modify: `servora/security/authz/authz_test.go`

- [ ] **Step 5.1: Write the failing test**

Append to `servora/security/authz/authz_test.go`:

```go
// blockingAuthorizer simulates a slow backend by waiting until ctx is cancelled.
type blockingAuthorizer struct{}

func (b *blockingAuthorizer) Check(ctx context.Context, _, _, _, _ string) (bool, error) {
	<-ctx.Done()
	return false, ctx.Err()
}
func (b *blockingAuthorizer) BatchCheck(ctx context.Context, _ []CheckRequest) ([]CheckResult, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (b *blockingAuthorizer) ListAllowed(ctx context.Context, _, _, _ string) ([]string, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// TestServer_CheckTimeout_TripsCheckBeforeBackend ensures Check is bounded.
func TestServer_CheckTimeout_TripsCheckBeforeBackend(t *testing.T) {
	mw := Server(
		&blockingAuthorizer{},
		WithRules(map[string]AuthzRule{
			testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Relation: "admin", ObjectType: "platform"},
		}),
		WithCheckTimeout(50*time.Millisecond),
	)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler must not be reached when check times out")
		return nil, nil
	})

	ctx := userActorCtx(transportCtx(testOp), "user-123")
	start := time.Now()
	_, err := handler(ctx, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed >= 500*time.Millisecond {
		t.Errorf("elapsed = %v, expected < 500ms (timeout should trip well before)", elapsed)
	}
}
```

Add `"time"` to the imports of `authz_test.go` if not already there.

- [ ] **Step 5.2: Run the test (expect FAIL on undefined `WithCheckTimeout`)**

Run: `cd servora && go test ./security/authz/ -run TestServer_CheckTimeout -v`
Expected: FAIL — `undefined: WithCheckTimeout`.

- [ ] **Step 5.3: Implement the option**

Edit `servora/security/authz/authz.go`.

Add `"time"` to imports.

Add field to `serverConfig`:
```go
type serverConfig struct {
	rules          map[string]AuthzRule
	defaultObjID   string
	decisionLogger func(ctx context.Context, detail DecisionDetail)
	checkTimeout   time.Duration
}
```

Add option function (next to other `WithXxx`):
```go
// WithCheckTimeout bounds the time spent in Authorizer.Check on each request.
// Zero (default) disables the deadline — the upstream context applies.
//
// This protects business-RPC latency from a slow authorization backend
// (e.g. OpenFGA cross-region calls).
func WithCheckTimeout(d time.Duration) Option {
	return func(cfg *serverConfig) { cfg.checkTimeout = d }
}
```

Update the call to `authorizer.Check` in `Server()`:
```go
checkCtx := ctx
if cfg.checkTimeout > 0 {
	var cancel context.CancelFunc
	checkCtx, cancel = context.WithTimeout(ctx, cfg.checkTimeout)
	defer cancel()
}

allowed, err := authorizer.Check(checkCtx, principal, relation, objectType, objectID)
```

(Replace the previous single-line call.)

- [ ] **Step 5.4: Run the test (expect PASS)**

Run: `cd servora && go test ./security/authz/ -run TestServer_CheckTimeout -v -race`
Expected: PASS in <500ms.

- [ ] **Step 5.5: Run full authz suite for regression**

Run: `cd servora && go test ./security/authz/... -race`
Expected: PASS.

- [ ] **Step 5.6: Commit**

```bash
cd servora
git add security/authz/authz.go security/authz/authz_test.go
git commit -m "feat(security/authz): add WithCheckTimeout option to bound backend latency"
```

---

## Task 6: Add `WithFailOpenOnMissingRule` option

**Why:** Strict fail-closed behavior is correct for production but brutal during development — a forgotten rule blacks out the new RPC. This option turns missing-rule into a logged warning + handler-passthrough, with an alert callback so it's visible.

**Files:**
- Modify: `servora/security/authz/authz.go`
- Modify: `servora/security/authz/authz_test.go`

- [ ] **Step 6.1: Write failing test**

Append to `authz_test.go`:

```go
// TestServer_FailOpenOnMissingRule_PassesThroughAndAlerts verifies the option.
func TestServer_FailOpenOnMissingRule_PassesThroughAndAlerts(t *testing.T) {
	var alerted *string
	mw := Server(nil,
		// no rules
		WithFailOpenOnMissingRule(func(ctx context.Context, operation string) {
			alerted = &operation
		}),
	)

	called := false
	handler := mw(func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	})

	ctx := transportCtx(testOp)
	resp, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("expected pass-through, got err=%v", err)
	}
	if !called {
		t.Fatal("handler must be called when fail-open is on")
	}
	if resp != "ok" {
		t.Errorf("resp = %v, want ok", resp)
	}
	if alerted == nil || *alerted != testOp {
		t.Errorf("alert callback not invoked with operation %q (got %v)", testOp, alerted)
	}
}

// TestServer_NoFailOpen_StillFailsClosed ensures default behavior is unchanged.
func TestServer_NoFailOpen_StillFailsClosed(t *testing.T) {
	mw := Server(nil) // no rules, no fail-open option
	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler must not be called by default")
		return nil, nil
	})

	ctx := transportCtx(testOp)
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected fail-closed error for missing rule")
	}
}
```

- [ ] **Step 6.2: Run the test (expect FAIL)**

Run: `cd servora && go test ./security/authz/ -run TestServer_FailOpen -v`
Expected: FAIL — `undefined: WithFailOpenOnMissingRule`.

- [ ] **Step 6.3: Implement the option**

Edit `servora/security/authz/authz.go`.

Add to `serverConfig`:
```go
type serverConfig struct {
	rules              map[string]AuthzRule
	defaultObjID       string
	decisionLogger     func(ctx context.Context, detail DecisionDetail)
	checkTimeout       time.Duration
	missingRuleAlertFn func(ctx context.Context, operation string)
}
```

Add option:
```go
// WithFailOpenOnMissingRule changes the missing-rule policy from fail-closed
// (default — return AUTHZ_NO_RULE 403) to fail-open: the handler is called,
// and the alertFn callback is invoked so the gap is visible (oncall page,
// Slack, log warning, etc.).
//
// Use during development or staged rollouts. NEVER use in production for
// security-sensitive services.
func WithFailOpenOnMissingRule(alertFn func(ctx context.Context, operation string)) Option {
	return func(cfg *serverConfig) { cfg.missingRuleAlertFn = alertFn }
}
```

Update the missing-rule branch in `Server()`. Find:
```go
rule, found := cfg.rules[operation]
if !found {
	return nil, errors.Forbidden("AUTHZ_NO_RULE",
		fmt.Sprintf("no authorization rule for operation %s", operation))
}
```

Replace with:
```go
rule, found := cfg.rules[operation]
if !found {
	if cfg.missingRuleAlertFn != nil {
		cfg.missingRuleAlertFn(ctx, operation)
		return handler(ctx, req)
	}
	return nil, errors.Forbidden("AUTHZ_NO_RULE",
		fmt.Sprintf("no authorization rule for operation %s", operation))
}
```

- [ ] **Step 6.4: Run the test (expect PASS)**

Run: `cd servora && go test ./security/authz/ -run TestServer_FailOpen -v -race`
Expected: PASS.

- [ ] **Step 6.5: Run full authz suite**

Run: `cd servora && go test ./security/authz/... -race`
Expected: PASS.

- [ ] **Step 6.6: Commit**

```bash
cd servora
git add security/authz/authz.go security/authz/authz_test.go
git commit -m "feat(security/authz): add WithFailOpenOnMissingRule for dev-time ergonomics"
```

---

## Task 7: Support nested IDField (dot-path)

**Why:** Today `extractProtoField` only resolves top-level scalar fields. Nested messages (`parent.id`) are common in APIs — supporting them keeps proto annotations expressive without forcing flat request shapes.

**Constraints (intentional):**
- Each path segment must be a singular (non-repeated) field
- Path terminus must be a scalar (string / int family); message terminus is an error
- Empty values still error (existing contract)

**Files:**
- Modify: `servora/security/authz/authz.go` (function `extractProtoField`)
- Modify: `servora/security/authz/authz_test.go`

- [ ] **Step 7.1: Add `structpb` import to `authz_test.go`**

Add to existing `authz_test.go` import block:
```go
structpb "google.golang.org/protobuf/types/known/structpb"
```

- [ ] **Step 7.2: Write failing tests**

Append to `authz_test.go`:

```go
// TestExtractProtoField_DotPath_NestedScalar resolves a nested scalar via path.
// We use structpb.Struct because its fields map<string, Value> + Value.string_value
// gives us a real two-level proto path without inventing a custom message.
func TestExtractProtoField_DotPath_NestedScalar(t *testing.T) {
	req, err := structpb.NewStruct(map[string]interface{}{
		"id": "outer-123",
	})
	if err != nil {
		t.Fatalf("structpb: %v", err)
	}

	got, err := extractProtoField(req, "fields.id.string_value")
	if err != nil {
		t.Fatalf("extractProtoField err = %v", err)
	}
	if got != "outer-123" {
		t.Errorf("got %q, want outer-123", got)
	}
}

// TestExtractProtoField_DotPath_MissingSegment errors out cleanly.
func TestExtractProtoField_DotPath_MissingSegment(t *testing.T) {
	req, _ := structpb.NewStruct(map[string]interface{}{"id": "x"})
	_, err := extractProtoField(req, "fields.missing.string_value")
	if err == nil {
		t.Fatal("expected error for missing nested segment")
	}
}

// TestExtractProtoField_DotPath_TerminatesOnMessage_Errors guards against
// silently String()-ifying a message into textproto garbage.
func TestExtractProtoField_DotPath_TerminatesOnMessage_Errors(t *testing.T) {
	req, _ := structpb.NewStruct(map[string]interface{}{"id": "x"})
	_, err := extractProtoField(req, "fields.id") // terminates on Value (a message)
	if err == nil {
		t.Fatal("expected error when path terminus is a message, not a scalar")
	}
}

// TestExtractProtoField_TopLevel_StillWorks ensures backwards compatibility.
func TestExtractProtoField_TopLevel_StillWorks(t *testing.T) {
	req := &wrapperspb.StringValue{Value: "user-abc"}
	got, err := extractProtoField(req, "value")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "user-abc" {
		t.Errorf("got %q, want user-abc", got)
	}
}
```

- [ ] **Step 7.3: Run tests (expect FAIL on dot-path cases)**

Run: `cd servora && go test ./security/authz/ -run TestExtractProtoField_DotPath -v`
Expected: FAIL on the three dot-path tests; the top-level test passes today.

- [ ] **Step 7.4: Reimplement `extractProtoField` with dot-path support**

Edit `servora/security/authz/authz.go`. Add `"strings"` to imports.

Replace the existing `extractProtoField`:

```go
func extractProtoField(req any, fieldPath string) (string, error) {
	if fieldPath == "" {
		return "", fmt.Errorf("id_field not specified")
	}
	msg, ok := req.(proto.Message)
	if !ok {
		return "", fmt.Errorf("request is not a proto message")
	}

	segments := strings.Split(fieldPath, ".")
	current := msg.ProtoReflect()

	for i, seg := range segments {
		fd := current.Descriptor().Fields().ByName(protoreflect.Name(seg))
		if fd == nil {
			return "", fmt.Errorf("field %q not found in %s",
				seg, current.Descriptor().FullName())
		}
		if fd.IsList() || fd.IsMap() {
			return "", fmt.Errorf("field %q is repeated/map; not supported in id_field path",
				seg)
		}

		isLast := i == len(segments)-1
		val := current.Get(fd)

		if !isLast {
			// Must be a singular message to traverse further.
			if fd.Kind() != protoreflect.MessageKind {
				return "", fmt.Errorf("path segment %q is scalar but path continues", seg)
			}
			current = val.Message()
			continue
		}

		// Last segment must be a scalar.
		if fd.Kind() == protoreflect.MessageKind {
			return "", fmt.Errorf("path %q terminates on a message field; expected scalar",
				fieldPath)
		}
		s := val.String()
		if s == "" {
			return "", fmt.Errorf("field %q is empty", fieldPath)
		}
		return s, nil
	}

	return "", fmt.Errorf("unreachable: empty path segments")
}
```

- [ ] **Step 7.5: Run dot-path tests (expect PASS)**

Run: `cd servora && go test ./security/authz/ -run TestExtractProtoField -v -race`
Expected: PASS on all four tests (three dot-path + top-level backward-compat).

- [ ] **Step 7.6: Run full authz suite**

Run: `cd servora && go test ./security/authz/... -race`
Expected: PASS.

- [ ] **Step 7.7: Commit**

```bash
cd servora
git add security/authz/authz.go security/authz/authz_test.go
git commit -m "feat(security/authz): support dot-path id_field for nested proto messages"
```

---

## Task 8: Audit bridge — `NewAuthzBridge(recorder)` (closes P0-2)

**Why:** Today every business has to manually wire `WithDecisionLogger(func(ctx, d) { recorder.RecordAuthzDecision(...) })` — easy to forget. Provide a one-liner.

**Files:**
- Create: `servora/security/authz/bridge.go`
- Create: `servora/security/authz/bridge_test.go`

- [ ] **Step 8.1: Write failing test**

Create `servora/security/authz/bridge_test.go`:

```go
package authz

import (
	"context"
	"testing"

	"github.com/Servora-Kit/servora/obs/audit"
)

// captureEmitter records every event emitted, for assertion.
type captureEmitter struct {
	events []*audit.AuditEvent
}

func (c *captureEmitter) Emit(_ context.Context, e *audit.AuditEvent) error {
	c.events = append(c.events, e)
	return nil
}
func (c *captureEmitter) Close() error { return nil }

// TestNewAuthzBridge_AllowedDecisionRecordsAuthzEvent verifies wire-up.
func TestNewAuthzBridge_AllowedDecisionRecordsAuthzEvent(t *testing.T) {
	emitter := &captureEmitter{}
	recorder := audit.NewRecorder(emitter, "test-svc")
	bridge := NewAuthzBridge(recorder)

	ctx := userActorCtx(transportCtx(testOp), "user-123")
	bridge(ctx, DecisionDetail{
		Operation:  testOp,
		Subject:    "user:user-123",
		Relation:   "viewer",
		ObjectType: "doc",
		ObjectID:   "doc-1",
		Allowed:    true,
	})

	if len(emitter.events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(emitter.events))
	}
	evt := emitter.events[0]
	if evt.EventType != audit.EventTypeAuthzDecision {
		t.Errorf("evt.EventType = %s, want %s", evt.EventType, audit.EventTypeAuthzDecision)
	}
	if evt.Operation != testOp {
		t.Errorf("evt.Operation = %s, want %s", evt.Operation, testOp)
	}
	detail, ok := evt.Detail.(audit.AuthzDetail)
	if !ok {
		t.Fatalf("evt.Detail = %T, want audit.AuthzDetail", evt.Detail)
	}
	if detail.Decision != audit.AuthzDecisionAllowed {
		t.Errorf("detail.Decision = %s, want allowed", detail.Decision)
	}
	if detail.ObjectID != "doc-1" {
		t.Errorf("detail.ObjectID = %s, want doc-1", detail.ObjectID)
	}
}

// TestNewAuthzBridge_DeniedDecisionMapsToDenied checks decision mapping.
func TestNewAuthzBridge_DeniedDecisionMapsToDenied(t *testing.T) {
	emitter := &captureEmitter{}
	recorder := audit.NewRecorder(emitter, "test-svc")
	bridge := NewAuthzBridge(recorder)

	ctx := userActorCtx(transportCtx(testOp), "user-123")
	bridge(ctx, DecisionDetail{
		Operation: testOp, Subject: "user:user-123", Relation: "admin",
		ObjectType: "platform", ObjectID: "default", Allowed: false,
	})

	if len(emitter.events) != 1 {
		t.Fatalf("len(events) = %d", len(emitter.events))
	}
	detail := emitter.events[0].Detail.(audit.AuthzDetail)
	if detail.Decision != audit.AuthzDecisionDenied {
		t.Errorf("detail.Decision = %s, want denied", detail.Decision)
	}
}

// TestNewAuthzBridge_ErrorMapsToErrorDecision uses Err to mean engine failure.
func TestNewAuthzBridge_ErrorMapsToErrorDecision(t *testing.T) {
	emitter := &captureEmitter{}
	recorder := audit.NewRecorder(emitter, "test-svc")
	bridge := NewAuthzBridge(recorder)

	ctx := userActorCtx(transportCtx(testOp), "user-123")
	bridge(ctx, DecisionDetail{
		Operation: testOp, Subject: "user:user-123",
		Relation: "x", ObjectType: "y", ObjectID: "z",
		Err: context.DeadlineExceeded,
	})

	detail := emitter.events[0].Detail.(audit.AuthzDetail)
	if detail.Decision != audit.AuthzDecisionError {
		t.Errorf("detail.Decision = %s, want error", detail.Decision)
	}
	if detail.ErrorReason == "" {
		t.Errorf("detail.ErrorReason should be populated on Err")
	}
}

// TestNewAuthzBridge_NilRecorder_NoOp ensures safe wiring with nil.
func TestNewAuthzBridge_NilRecorder_NoOp(t *testing.T) {
	bridge := NewAuthzBridge(nil)
	// Must not panic.
	bridge(context.Background(), DecisionDetail{Operation: testOp, Allowed: true})
}
```

- [ ] **Step 8.2: Run test (expect FAIL)**

Run: `cd servora && go test ./security/authz/ -run TestNewAuthzBridge -v`
Expected: FAIL — `undefined: NewAuthzBridge`.

- [ ] **Step 8.3: Implement the bridge**

Create `servora/security/authz/bridge.go`:

```go
package authz

import (
	"context"

	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/obs/audit"
)

// NewAuthzBridge returns a DecisionLogger that forwards every authorization
// decision to the audit Recorder as an AUTHZ_DECISION event.
//
// Use it as a one-liner in middleware setup:
//
//	authz.Server(authorizer,
//	    authz.WithRulesFunc(rules),
//	    authz.WithDecisionLogger(authz.NewAuthzBridge(recorder)),
//	)
//
// Closes TODO P0-2 (authz → audit auto-bridge).
//
// If recorder is nil, the returned function is a safe no-op.
func NewAuthzBridge(recorder *audit.Recorder) func(context.Context, DecisionDetail) {
	if recorder == nil {
		return func(context.Context, DecisionDetail) {}
	}
	return func(ctx context.Context, d DecisionDetail) {
		a, _ := actor.FromContext(ctx)
		recorder.RecordAuthzDecision(ctx, d.Operation, a, audit.AuthzDetail{
			Relation:    d.Relation,
			ObjectType:  d.ObjectType,
			ObjectID:    d.ObjectID,
			Decision:    decisionFromDetail(d),
			ErrorReason: errorReasonFromDetail(d),
		})
	}
}

func decisionFromDetail(d DecisionDetail) audit.AuthzDecision {
	switch {
	case d.Err != nil:
		return audit.AuthzDecisionError
	case d.Allowed:
		return audit.AuthzDecisionAllowed
	default:
		return audit.AuthzDecisionDenied
	}
}

func errorReasonFromDetail(d DecisionDetail) string {
	if d.Err == nil {
		return ""
	}
	return d.Err.Error()
}
```

- [ ] **Step 8.4: Run bridge tests (expect PASS)**

Run: `cd servora && go test ./security/authz/ -run TestNewAuthzBridge -v -race`
Expected: PASS on all four tests.

- [ ] **Step 8.5: Run full authz + audit suites**

Run: `cd servora && go test ./security/authz/... ./obs/audit/... -race`
Expected: PASS.

- [ ] **Step 8.6: Commit**

```bash
cd servora
git add security/authz/bridge.go security/authz/bridge_test.go
git commit -m "feat(security/authz): add NewAuthzBridge for one-liner audit wiring (closes P0-2)"
```

---

## Task 9: Package doc with contextual-tuples future note

**Why:** Tasks 1-8 don't actually implement contextual tuples (YAGNI), but a discoverable note in the package docs prevents future agents from re-deriving the design.

**Files:**
- Create: `servora/security/authz/doc.go`

- [ ] **Step 9.1: Create the file**

Create `servora/security/authz/doc.go`:

```go
// Package authz provides a Kratos middleware for relationship-based authorization.
//
// # Engine model
//
// The Authorizer interface (Check / BatchCheck / ListAllowed) maps directly onto
// OpenFGA SDK and SpiceDB primitives. Both ReBAC backends support all three
// methods natively. The interface is not designed to host non-ReBAC engines
// (Cedar, Rego); those would require a separate abstraction.
//
// # Future: contextual tuples
//
// OpenFGA's "contextual tuples" (and SpiceDB's "caveats") express request-level
// facts that participate in a decision but are not persisted: device trust,
// active session, time-of-day, request region, etc.
//
// When this is needed, the planned API is:
//
//	ctx = authz.WithContextualTuples(ctx, tuples...)        // upstream mw injects
//	authz.ContextualTuplesFromContext(ctx) []Tuple          // engine adapter reads
//
// The Authorizer interface signatures already accept context.Context as the
// first parameter, so no signature change will be required when this is added.
//
// # Audit integration
//
// Use authz.NewAuthzBridge(recorder) to forward every decision to obs/audit
// without per-call wiring.
package authz
```

- [ ] **Step 9.2: Verify it compiles**

Run: `cd servora && go vet ./security/authz/`
Expected: no output (clean).

- [ ] **Step 9.3: Commit**

```bash
cd servora
git add security/authz/doc.go
git commit -m "docs(security/authz): add package doc with contextual-tuples roadmap note"
```

---

## Task 10: Update TODO.md and AGENTS.md

**Why:** Plan completion is tracked in `docs/TODO.md` per its own conventions ("完成一项后将该条目移至「已完成」段，保留 1-2 行结论 + 关联 PR/commit").

**Files:**
- Modify: `servora/docs/TODO.md`
- Modify: `servora/security/authz/AGENTS.md`

- [ ] **Step 10.1: Edit TODO.md — remove P0-2 from active list**

In `servora/docs/TODO.md` (under `## P0`), find and delete the entire `### [P0-2]` block:

```markdown
### [P0-2] Authz 决策未自动桥接到 Audit Recorder

- **现状**：`security/authz/authz.go:199-201` 提供了 `WithDecisionLogger` 回调，但需要业务层手动转发到 `audit.Recorder`，极易遗漏。
- **建议**：
  1. 在 middleware 栈中默认注册 audit bridge，自动把每次授权决策记录为 `AUDIT_EVENT_TYPE_AUTHZ_DECISION`
  2. 提供 `audit.NewAuthzBridge(recorder)` 便利函数
  3. 文档说明如何关闭（如出于性能考虑）
```

- [ ] **Step 10.2: Edit TODO.md — append to 已完成 section**

Under `## 已完成` (after the existing `[P2-1b]` block, before `## 维护说明`), append:

```markdown
### [P0-2] Authz 决策自动桥接到 Audit Recorder ✅ 2026-05-01

`security/authz` 新增 `NewAuthzBridge(recorder)`，业务一行 `WithDecisionLogger(authz.NewAuthzBridge(recorder))` 即可把所有授权决策（allowed/denied/error）自动转 `audit.AuthzDetail` 落盘。Decision/Err 状态映射统一为 `AuthzDecisionAllowed/Denied/Error`。详见 [`superpowers/plans/2026-05-01-authz-closure-and-hardening.md`](superpowers/plans/2026-05-01-authz-closure-and-hardening.md)。

### [Authz 闭环 + 加固] ✅ 2026-05-01

同一 plan 一并完成：
- `Authorizer` 接口扩到三方法（`Check` / `BatchCheck` / `ListAllowed`），openfga 实现命中 SDK BatchCheck/ListObjects
- `DecisionDetail.CacheHit` 与 `audit.AuthzDetail.CacheHit` 字段删除（cache 是 `infra/openfga` 内部优化，不进审计语义）
- `WithCheckTimeout` option（保护业务 RPC 不被慢授权后端拖垮）
- `WithFailOpenOnMissingRule` option（开发期友好，缺规则不再黑屏，alert 回调可见）
- `extractProtoField` 支持 dot-path（`parent.id` 等嵌套字段）
- 新增 `doc.go` 记录 contextual tuples 的未来路线（暂不实现）

详见 [`superpowers/plans/2026-05-01-authz-closure-and-hardening.md`](superpowers/plans/2026-05-01-authz-closure-and-hardening.md)。
```

- [ ] **Step 10.3: Edit `security/authz/AGENTS.md`**

Append to the existing `## 当前实现事实` section:

```markdown
- `Authorizer` 接口含三方法：`Check` / `BatchCheck` / `ListAllowed`，openfga 与 noop 完整覆盖
- `WithCheckTimeout(d)` 限制后端调用时长；`WithFailOpenOnMissingRule(alertFn)` 开发期可放行未注册 RPC 并回调告警
- `extractProtoField` 支持 dot-path（`parent.id`），路径中段必须为单 message，终点必须为标量
- `NewAuthzBridge(recorder)` 一键把 decision 落到 `obs/audit` Recorder
```

- [ ] **Step 10.4: Commit**

```bash
cd servora
git add docs/TODO.md security/authz/AGENTS.md
git commit -m "docs: mark P0-2 done; record authz closure & hardening plan completion"
```

---

## Task 11: Final verification

- [ ] **Step 11.1: Full build with race detector**

Run: `cd servora && go test ./... -race -count=1`
Expected: PASS — every package, no flake.

- [ ] **Step 11.2: Lint**

Run: `cd servora && make ci.lint`
Expected: PASS.

- [ ] **Step 11.3: Coverage spot-check on touched packages**

Run: `cd servora && go test ./security/authz/... ./infra/openfga/... ./obs/audit/... -cover`
Expected: each package ≥80% coverage. If `security/authz` drops below, add cases for the new branches (timeout / fail-open / dot-path / bridge).

- [ ] **Step 11.4: Verify no `IsAuthorized` leftovers**

Run: `cd servora && grep -rn "IsAuthorized" --include="*.go" .`
Expected: zero matches (only Check / BatchCheck / ListAllowed in the new interface).

- [ ] **Step 11.5: Verify no `CacheHit` leftovers in semantic layers**

Run: `cd servora && grep -rn "CacheHit" --include="*.go" obs/audit security/authz`
Expected: zero matches in `obs/audit` and `security/authz`. Matches inside `infra/openfga/cache.go` (the `cacheHit` return from `CachedCheck`) are expected and stay — that's the cache layer's own concern.

- [ ] **Step 11.6: Tag (only if maintainer approves bump)**

This plan introduces a breaking API change in `security/authz`. Coordinator decides version bump. Then:

```bash
cd servora
make tag TAG=v0.3.0     # or whatever the agreed bump is
```

(Do **not** run `make tag.api` — proto definitions in `api/protos/` are unchanged.)

---

## Self-Review Notes

**Spec coverage:**
- ✅ Authorizer interface 3 methods → Task 4
- ✅ DecisionDetail.CacheHit removed → Task 3
- ✅ audit.AuthzDetail.CacheHit removed → Task 1
- ✅ NewAuthzBridge (P0-2) → Task 8
- ✅ WithCheckTimeout → Task 5
- ✅ WithFailOpenOnMissingRule → Task 6
- ✅ Dot-path IDField → Task 7
- ✅ doc.go contextual-tuples note → Task 9
- ✅ TODO.md update → Task 10

**Out-of-scope (confirmed not done):**
- Contextual tuples implementation (deferred — YAGNI)
- SHADOW mode / proto enum changes (per user)
- Metrics (per user)
- Subject mapper (framework convention)
- Recovery wrapper (kratos-provided)
- Cache code deletion (kept as opt-in)

**Type consistency check:**
- `CheckRequest{Subject, Relation, ObjectType, ObjectID}` (in `security/authz`) vs `BatchCheckItem{User, Relation, Object}` (in `infra/openfga`) — different on purpose: the authz layer keeps `(ObjectType, ObjectID)` separate so middleware doesn't pre-format; the openfga client layer flattens to `Object: type+":"+id`. Mapping happens in `openfga.Authorizer.BatchCheck` (Task 4.7).
- `CheckResult{Allowed, Err}` is consistent across `infra/openfga` and `security/authz` layers.
- `decisionFromDetail` mapping: `Err != nil → Error`, `Allowed → Allowed`, else `Denied`. No `NoRule` decision here because the bridge is only called when a decision was attempted (no-rule path returns earlier in middleware before logger fires).
- `BatchCheckItem.User` (openfga client field) ↔ `CheckRequest.Subject` (authz interface field) — mapped 1:1 in Task 4.7's loop.

**Placeholder scan:** No "TBD", no "implement later", no "similar to Task N" — all code blocks are complete.
