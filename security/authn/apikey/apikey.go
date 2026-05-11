// Package apikey provides an API-key authentication skeleton for the
// engine-agnostic [authn] dispatcher. It is a thin engine: the framework
// owns the wiring (header extraction + dispatch) while the storage backend
// (in-memory / DB / cache / cross-service RPC) is supplied by the business
// via the [Store] interface.
//
// Header carrier:
//
//   - The engine reads the API key from the inbound `X-API-Key` header on
//     the Kratos server transport.
//   - It deliberately does NOT consume `Authorization: Bearer <token>` so
//     that an apikey engine can be wired alongside `security/authn/jwt` in
//     the same `authn.Multi` decorator without header collision.
//
// Multi-engine composition example:
//
//	authn.Server(
//	    authn.Multi(
//	        authn.Named(jwt.Scheme,    jwt.NewAuthenticator(jwt.WithVerifier(jv))),
//	        authn.Named(apikey.Scheme, apikey.NewAuthenticator(apikey.WithStore(myStore))),
//	    ),
//	    authn.WithRulesFuncs(myProto.AuthnRules),
//	)
//
// Storage backend ownership:
//
//   - The framework imposes no constraint on how API keys are stored or
//     revoked; that is a business / platform concern.
//   - Implementations of [Store] may be backed by an in-memory map (test
//     stub), a database table, a cache, or a cross-service RPC.
//   - [Store.Lookup] returns a [KeyMeta] with the key identifier and owner
//     subject. On successful authentication, the engine writes [KeyMeta]
//     into ctx via [WithKeyMeta] and sets auth type to "api_key".
//
// Downstream code reads the result via:
//
//   - [KeyMetaFrom] — full metadata struct.
//   - [SubjectFrom] — convenience: extracts OwnerID only.
//   - [authn.AuthTypeFrom] — "api_key" on success.
//
// Unlike `security/authn/jwt`, this subpackage exposes no `ClaimsMapper`
// extension point: API keys carry no JWT claims and the key metadata is
// produced directly by [Store.Lookup].
package apikey

// Scheme is the canonical scheme string for this engine, paired with
// [NewAuthenticator] via [authn.Named]. The framework does not enumerate
// scheme constants — each engine sub-package owns its own string.
const Scheme = "apikey"
