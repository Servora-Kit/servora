package jwt

import "strings"

// extractBearerToken parses the Bearer token out of an Authorization header
// value. Returns "" if the header is absent or malformed.
//
// Migrated from the historical public `authn.ExtractBearerToken`: the
// framework main package no longer hosts credential-carrier parsing because
// "Bearer" is a jwt-shaped concept (mTLS reads peer certs, API-Key reads a
// different header, etc.). Lowercase first letter — package-private.
// Business code MUST NOT call this directly; if you want to extract a token,
// use the inbound [Server] middleware which calls this internally and stores
// the result via [WithToken].
func extractBearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}
