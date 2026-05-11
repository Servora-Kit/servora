// Package audit records cross-cutting audit events (authentication outcomes,
// authorization decisions, OpenFGA tuple changes, business resource mutations)
// into a unified auditpb.AuditEvent envelope and delivers them through a
// pluggable Emitter (broker, log, noop). Schema is sourced exclusively from
// api/gen/go/servora/audit/v1 — there is no runtime↔proto mapper.
//
// # API surface
//
// Three groups, top-down by typical caller:
//
//   1. Ctx helpers — written by security middleware, read by Collector:
//        WithAuthnResult(ctx, *auditpb.AuthnDetail) context.Context
//        AuthnResultFrom(ctx) (*auditpb.AuthnDetail, bool)
//        WithAuthzResult(ctx, *auditpb.AuthzDetail) context.Context
//        AuthzResultFrom(ctx) (*auditpb.AuthzDetail, bool)
//        InstallHolder(ctx) context.Context  // auto-called by Collector; rarely needed directly
//
//   2. Middleware — assembled by business apps:
//        Collector(rec *Recorder, opts ...CollectorOption) middleware.Middleware
//        WithSpanEvents(enabled bool) CollectorOption  // default true
//
//   3. Recorder — direct event emission for non-middleware paths:
//        NewRecorder(emitter Emitter, serviceName string) *Recorder
//        (*Recorder).Emit(ctx, *auditpb.AuditEvent) error
//        (*Recorder).RecordResourceMutation(...)   // proto-annotation-driven middleware uses this
//        (*Recorder).RecordTupleChange(...)        // security/authz/openfga write path uses this
//
// # Audit event sources topology
//
// The four auditpb.AuditEventType values originate from different modules:
//
//   AUTHN_RESULT       ← security/authn middleware writes ctx, Collector emits
//   AUTHZ_DECISION    ← security/authz middleware writes ctx, Collector emits
//   RESOURCE_MUTATION ← obs/audit middleware (proto-annotation driven, separate path)
//   TUPLE_CHANGED     ← security/authz/openfga write path (calls Recorder.RecordTupleChange directly)
//
// Each EventType's Result reflects only its own layer's outcome. Handler
// business errors are recorded via RESOURCE_MUTATION, never leak into
// AUTHN_RESULT or AUTHZ_DECISION — consumers can distinguish "authn failed"
// from "authn ok but business failed" by EventType alone.
//
// # Mounting rule (CRITICAL)
//
// Collector MUST be the OUTER-most middleware relative to authn / authz.
// Kratos middleware chains wrap inner LIFO; if authn fails and short-circuits,
// only an outer Collector reaches its post-phase to read ctx and emit. Listing
// Collector inner to authn silently drops failure events (no panic, no error).
//
// Recommended chain order:
//
//	recovery → tracing → logging → ratelimit → validate → metrics → audit.Collector → authn → authz → handler
//	                                                                ^^^^^^^^^^^^^^^
//	                                                                outer to authn/authz; trailing position
//	                                                                aligns with transport/server/middleware
//	                                                                ChainBuilder.Build output
//
// 本顺序与 transport/server/middleware.ChainBuilder 的 Build 输出对齐；调用
// WithAudit(rec) 即自动落到该位置，无需业务方手记 outer/inner。
//
// # Example
//
// 推荐：通过 ChainBuilder.WithAudit 一行装配（无需手记 outer/inner 顺序）：
//
//	recorder := audit.NewRecorder(emitter, "iam")
//	mw := middleware.NewChainBuilder(l).
//	    WithTrace(trace).
//	    WithMetrics(mtc).
//	    WithAudit(recorder).
//	    Build()
//	mw = append(mw,
//	    authn.Server(jwtAuth),
//	    authz.Server(fgaAuth, authz.WithRulesFunc(iampb.AuthzRules)),
//	)
//
// 如果不用 ChainBuilder 也可以手写——注意 audit.Collector 必须 OUTER 于 authn/authz：
//
//	mw := []middleware.Middleware{
//	    recovery.Recovery(),
//	    tracing.Server(),
//	    logging.Server(l),
//	    audit.Collector(recorder),
//	    authn.Server(jwtAuth),
//	    authz.Server(fgaAuth, authz.WithRulesFunc(iampb.AuthzRules)),
//	}
//
// See AGENTS.md in this package for the full mounting contract, push-ctx
// rationale, and emit pipeline.
package audit
