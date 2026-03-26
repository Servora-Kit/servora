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
type Actor interface {
	ID() string
	Type() Type
	DisplayName() string

	Email() string
	Subject() string
	ClientID() string
	Realm() string
	Roles() []string
	Scopes() []string
	Attrs() map[string]string
}
