package jwt

import "context"

// SubjectFrom is a convenience accessor that reads the "sub" claim from the
// jwt-private claims ctx channel. It returns ("", false) if no claims are
// present or if "sub" is missing or not a string.
func SubjectFrom(ctx context.Context) (string, bool) {
	claims, ok := ClaimsFrom(ctx)
	if !ok {
		return "", false
	}
	sub, ok := claims["sub"]
	if !ok {
		return "", false
	}
	s, ok := sub.(string)
	return s, ok
}
