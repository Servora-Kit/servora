package jwt

import (
	"fmt"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/Servora-Kit/servora/core/actor"
)

// ClaimsMapper converts parsed JWT MapClaims into an actor.Actor.
//
// This is the extension point business code uses to interpret IdP-specific
// claims. The framework ships only a minimal three-piece default — anything
// richer (custom roles, tenant, scopes, group memberships, …) belongs in
// business code, plugged via [WithClaimsMapper].
type ClaimsMapper func(claims gojwt.MapClaims) (actor.Actor, error)

// DefaultClaimsMapper returns the framework's minimal Bearer-JWT mapper.
//
// It maps ONLY the canonical JWT claims that every Bearer JWT carries to the
// actor three-piece (Type/ID/DisplayName):
//
//   - sub               → actor.UserActor.ID     (REQUIRED; empty → error)
//   - name              → actor.UserActor.DisplayName
//   - preferred_username → actor.UserActor.DisplayName  (fallback when name absent)
//
// IdP-specific claim interpretation (azp, scope, email, roles, groups,
// custom claims, …) is intentionally NOT covered. Business code that needs
// those fields installs its own ClaimsMapper via [WithClaimsMapper].
func DefaultClaimsMapper() ClaimsMapper {
	return mapDefaultClaims
}

func mapDefaultClaims(claims gojwt.MapClaims) (actor.Actor, error) {
	sub := claimString(claims, "sub")
	if sub == "" {
		return nil, fmt.Errorf("jwt: sub claim is empty")
	}
	name := claimString(claims, "name")
	if name == "" {
		name = claimString(claims, "preferred_username")
	}
	return actor.NewUserActor(sub, name), nil
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
