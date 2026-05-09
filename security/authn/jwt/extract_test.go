package jwt

import "testing"

// TestExtractBearerToken covers the full surface of extractBearerToken:
// scheme matching (case-insensitive), missing scheme, wrong scheme,
// whitespace handling around the token, and empty/missing inputs. The
// unit-level coverage complements TestServer_ExtractsBearerAndDispatches
// (which only exercises the happy path through the wrapper).
func TestExtractBearerToken(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   string
	}{
		{name: "empty-header", header: "", want: ""},
		{name: "canonical-bearer", header: "Bearer abc123", want: "abc123"},
		{name: "lowercase-scheme", header: "bearer abc123", want: "abc123"},
		{name: "uppercase-scheme", header: "BEARER abc123", want: "abc123"},
		{name: "double-space-stripped", header: "Bearer  abc123", want: "abc123"},
		{name: "scheme-only-trailing-space", header: "Bearer ", want: ""},
		{name: "scheme-only-no-space", header: "Bearer", want: ""},
		{name: "wrong-scheme-basic", header: "Basic xyz", want: ""},
		{name: "no-scheme-prefix", header: "abc123", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractBearerToken(tc.header)
			if got != tc.want {
				t.Errorf("extractBearerToken(%q) = %q, want %q", tc.header, got, tc.want)
			}
		})
	}
}
