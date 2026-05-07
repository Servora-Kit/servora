package audit

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/trace"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
)

// Span event names — co-located here for grep-friendliness across the audit pipeline.
//
// spanEventAuthnRecorded / spanEventAuthzRecorded are emitted at write time by
// security middleware (via WithAuthnResult / WithAuthzResult).
// spanEventCollected is emitted by audit.Collector at the chain tail after
// emission. Together they form the audit pipeline timeline on a single OTel span.
const (
	spanEventAuthnRecorded = "audit.authn.recorded"
	spanEventAuthzRecorded = "audit.authz.recorded"
	spanEventCollected     = "audit.collected"
)

// holderKey is the unexported sentinel ctx-key for the per-request detail holder.
type holderKey struct{}

// detailHolder is the mutable bag shared between push-ctx writers
// (security middleware) and the outer Collector reader. Stored in ctx
// as a pointer so writes propagate up the middleware chain despite
// Go's context.WithValue immutability.
//
// Why a holder, not direct WithValue: Kratos middleware Chain wraps inner
// LIFO. Outer middleware holds the original ctx; if inner does
// `ctx2 = context.WithValue(ctx, ...)` and calls handler(ctx2), only
// downstream sees ctx2. Outer's post-phase still has the ORIGINAL ctx
// without the value. Mutating a pointer threaded through the original
// ctx solves this — same trick as OTel SDK's trace.SpanFromContext.
//
// Thread-safety: a single request flows through middleware on a single
// goroutine; concurrent access shouldn't happen in practice. Mutex is
// included as belt-and-suspenders against future async fan-out (e.g.
// transcoding gateway) — cheap, prevents subtle races.
type detailHolder struct {
	mu    sync.Mutex
	authn *auditpb.AuthnDetail
	authz *auditpb.AuthzDetail
}

// InstallHolder mounts a fresh detail holder into ctx. Called by
// audit.Collector at the outermost position of the middleware chain.
// Must be called before any inner middleware invokes WithAuthnResult /
// WithAuthzResult; otherwise those writes are silently dropped.
//
// Idempotent: a second call on a ctx that already carries a holder is a
// no-op (returns the same ctx). This prevents a nested Collector from
// orphaning the outer holder's already-written details.
func InstallHolder(ctx context.Context) context.Context {
	if _, ok := ctx.Value(holderKey{}).(*detailHolder); ok {
		return ctx
	}
	return context.WithValue(ctx, holderKey{}, &detailHolder{})
}

// holderFrom returns the detail holder previously installed by InstallHolder,
// or nil if none. Internal helper.
func holderFrom(ctx context.Context) *detailHolder {
	h, _ := ctx.Value(holderKey{}).(*detailHolder)
	return h
}

// WithAuthnResult records an authentication outcome into the per-request
// holder previously installed by audit.Collector. Idempotent: latest call
// wins (a single request only authenticates once). Returns the same ctx
// (no child created); the holder mutation is what propagates upward.
//
// nil detail makes this a no-op. If the holder hasn't been installed
// (i.e. Collector isn't mounted as outer), this silently drops the write —
// the audit event won't be emitted. See obs/audit/AGENTS.md for the
// mounting contract.
//
// Also attaches an "audit.authn.recorded" event to the active OTel span
// (noop when no span is active). Span events fire only when the holder
// is present, so misconfigured chains don't pollute traces with phantom
// audit markers.
func WithAuthnResult(ctx context.Context, d *auditpb.AuthnDetail) context.Context {
	if d == nil {
		return ctx
	}
	h := holderFrom(ctx)
	if h == nil {
		return ctx
	}
	h.mu.Lock()
	h.authn = d
	h.mu.Unlock()
	trace.SpanFromContext(ctx).AddEvent(spanEventAuthnRecorded)
	return ctx
}

// AuthnResultFrom returns the authn detail recorded by the matching
// WithAuthnResult call within this request, if any. Returns (nil, false)
// when no holder is installed or no detail has been written.
func AuthnResultFrom(ctx context.Context) (*auditpb.AuthnDetail, bool) {
	h := holderFrom(ctx)
	if h == nil {
		return nil, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.authn == nil {
		return nil, false
	}
	return h.authn, true
}

// WithAuthzResult mirrors WithAuthnResult for the authz layer.
// See WithAuthnResult docstring for semantics & no-holder behavior.
func WithAuthzResult(ctx context.Context, d *auditpb.AuthzDetail) context.Context {
	if d == nil {
		return ctx
	}
	h := holderFrom(ctx)
	if h == nil {
		return ctx
	}
	h.mu.Lock()
	h.authz = d
	h.mu.Unlock()
	trace.SpanFromContext(ctx).AddEvent(spanEventAuthzRecorded)
	return ctx
}

// AuthzResultFrom mirrors AuthnResultFrom.
func AuthzResultFrom(ctx context.Context) (*auditpb.AuthzDetail, bool) {
	h := holderFrom(ctx)
	if h == nil {
		return nil, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.authz == nil {
		return nil, false
	}
	return h.authz, true
}
