package jwt

import (
	"context"
	"fmt"

	gojwt "github.com/golang-jwt/jwt/v5"
)

// ClaimsMapper converts parsed JWT MapClaims into an enriched context.
//
// The first parameter is the incoming ctx that the engine needs to write
// private ctx channels into. The returned context carries whatever the
// mapper chose to store (at minimum the full claims map via [WithClaims]).
//
// This is the extension point business code uses to interpret IdP-specific
// claims. The framework ships only a minimal default — anything richer
// (custom roles, tenant, scopes, group memberships, …) belongs in
// business code, plugged via [WithClaimsMapper].
type ClaimsMapper func(ctx context.Context, claims gojwt.MapClaims) (context.Context, error)

// DefaultClaimsMapper returns the framework's minimal Bearer-JWT mapper.
//
// It validates that the `sub` claim is present and non-empty (REQUIRED),
// then stores the full claims map into the jwt-private ctx channel via
// [WithClaims]. Downstream handlers read individual claims via [ClaimsFrom]
// or the convenience [SubjectFrom].
//
// IdP-specific claim interpretation (azp, scope, email, roles, groups,
// custom claims, …) is intentionally NOT covered. Business code that needs
// those fields installs its own ClaimsMapper via [WithClaimsMapper].
func DefaultClaimsMapper() ClaimsMapper {
	return mapDefaultClaims
}

func mapDefaultClaims(ctx context.Context, claims gojwt.MapClaims) (context.Context, error) {
	sub := claimString(claims, "sub")
	if sub == "" {
		return ctx, fmt.Errorf("jwt: sub claim is empty")
	}
	return WithClaims(ctx, claims), nil
}

// claimString reads a single claim as a string. Numeric claims (float64 in
// gojwt MapClaims) are formatted as integers; other types use the default
// fmt %v form. Missing claims yield "".
func claimString(claims gojwt.MapClaims, key string) string {
	v, ok := claims[key]
	if !ok {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%.0f", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}
