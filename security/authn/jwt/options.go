package jwt

import jwtpkg "github.com/Servora-Kit/servora/security/jwt"

// Option configures the JWT Authenticator.
type Option func(*authenticatorConfig)

type authenticatorConfig struct {
	verifier     *jwtpkg.Verifier
	claimsMapper ClaimsMapper
}

// WithVerifier sets the JWT verifier used to validate token signatures.
// If nil, the authenticator operates in pass-through mode (anonymous actor
// returned without an error).
func WithVerifier(v *jwtpkg.Verifier) Option {
	return func(c *authenticatorConfig) { c.verifier = v }
}

// WithClaimsMapper sets a custom ClaimsMapper to convert JWT claims into
// an actor.Actor. Defaults to [DefaultClaimsMapper], which maps only the
// canonical three-piece (sub → ID, name/preferred_username → DisplayName).
//
// Business code that needs IdP-specific fields (custom roles / scopes /
// tenant / group memberships / …) installs its own mapper here.
func WithClaimsMapper(m ClaimsMapper) Option {
	return func(c *authenticatorConfig) { c.claimsMapper = m }
}
