package jwt

import (
	"fmt"
	"strings"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/Servora-Kit/servora/core/actor"
)

// ClaimsMapper converts parsed JWT MapClaims into an actor.Actor.
type ClaimsMapper func(claims gojwt.MapClaims) (actor.Actor, error)

// DefaultClaimsMapper maps standard OIDC claims (sub, name, email, azp, scope).
// It does not contain any IdP-specific fields (no issuer→Realm mapping).
func DefaultClaimsMapper() ClaimsMapper {
	return mapDefaultClaims
}

func mapDefaultClaims(claims gojwt.MapClaims) (actor.Actor, error) {
	sub := claimString(claims, "sub")
	id := sub
	if id == "" {
		id = claimString(claims, "id")
	}

	roles := claimStringSlice(claims, "roles")
	if singleRole := claimString(claims, "role"); singleRole != "" {
		roles = append(roles, singleRole)
	}

	attrs := make(map[string]string)
	if role := claimString(claims, "role"); role != "" {
		attrs["role"] = role
	}

	return actor.NewUserActor(actor.UserActorParams{
		ID:          id,
		DisplayName: claimString(claims, "name"),
		Email:       claimString(claims, "email"),
		Subject:     sub,
		ClientID:    claimString(claims, "azp"),
		Roles:       roles,
		Scopes:      claimStringSlice(claims, "scope"),
		Attrs:       attrs,
	}), nil
}

// KeycloakClaimsMapper extends DefaultClaimsMapper with Keycloak-specific field mappings:
// iss → Realm, realm_access.roles supplemental roles.
func KeycloakClaimsMapper() ClaimsMapper {
	return mapKeycloakClaims
}

func mapKeycloakClaims(claims gojwt.MapClaims) (actor.Actor, error) {
	a, err := mapDefaultClaims(claims)
	if err != nil {
		return nil, err
	}

	ua, ok := a.(*actor.UserActor)
	if !ok {
		return a, nil
	}

	realm := claimString(claims, "iss")
	roles := ua.Roles()
	if realmRoles := keycloakRealmRoles(claims); len(realmRoles) > 0 {
		seen := make(map[string]struct{}, len(roles))
		for _, r := range roles {
			seen[r] = struct{}{}
		}
		for _, r := range realmRoles {
			if _, dup := seen[r]; !dup {
				roles = append(roles, r)
			}
		}
	}

	return actor.NewUserActor(actor.UserActorParams{
		ID:          ua.ID(),
		DisplayName: ua.DisplayName(),
		Email:       ua.Email(),
		Subject:     ua.Subject(),
		ClientID:    ua.ClientID(),
		Realm:       realm,
		Roles:       roles,
		Scopes:      ua.Scopes(),
		Attrs:       ua.Attrs(),
	}), nil
}

// keycloakRealmRoles extracts roles from the Keycloak-specific
// "realm_access" -> "roles" nested claim.
func keycloakRealmRoles(claims gojwt.MapClaims) []string {
	ra, ok := claims["realm_access"]
	if !ok {
		return nil
	}
	raMap, ok := ra.(map[string]any)
	if !ok {
		return nil
	}
	return claimStringSlice(gojwt.MapClaims(raMap), "roles")
}

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

func claimStringSlice(claims gojwt.MapClaims, key string) []string {
	v, ok := claims[key]
	if !ok {
		return nil
	}
	switch val := v.(type) {
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if val == "" {
			return nil
		}
		return strings.Fields(val)
	default:
		return nil
	}
}
