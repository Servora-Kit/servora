package actor

// Type identifies the kind of request initiator (generic identity, not domain model).
type Type string

const (
	TypeUser      Type = "user"
	TypeSystem    Type = "system"
	TypeAnonymous Type = "anonymous"
	TypeService   Type = "service"
)

// Actor represents the identity of a request initiator.
// Scope is a generic key-value bag for request-scope dimensions (e.g. tenant/org/project IDs
// from gateway headers); keys are platform convention, not full domain model.
type Actor interface {
	ID() string
	Type() Type
	DisplayName() string

	// Identity fields sourced from IdP / OAuth2 token.
	Email() string
	Subject() string        // External IdP subject (e.g. Keycloak sub claim)
	ClientID() string       // OAuth2 client_id
	Realm() string          // IdP realm / tenant namespace
	Roles() []string        // Role list from token
	Scopes() []string       // OAuth2 scopes from token
	Attrs() map[string]string // Open extension bag for additional claims

	// Scope is a request-scope dimension bag (tenant/org/project IDs from gateway headers).
	// Not to be confused with OAuth2 scopes — this is Servora's request-scoping model.
	Scope(key string) string
}
