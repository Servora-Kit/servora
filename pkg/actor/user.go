package actor

// Scope key constants: conventional request-scope dimensions used by gateway and IAM.
// They are not the full domain model — only the keys used to pass scope from headers.
const (
	ScopeKeyTenantID       = "tenant_id"
	ScopeKeyOrganizationID = "organization_id"
	ScopeKeyProjectID      = "project_id"
)

// UserActorParams holds all fields for constructing a UserActor.
// New fields should be added here rather than extending the constructor signature.
type UserActorParams struct {
	ID          string
	DisplayName string
	Email       string
	Subject     string            // External IdP subject (Keycloak sub)
	ClientID    string            // OAuth2 client_id
	Realm       string            // IdP realm / tenant namespace
	Roles       []string          // Roles from token
	Scopes      []string          // OAuth2 scopes from token
	Attrs       map[string]string // Open extension bag
	Metadata    map[string]string // Arbitrary metadata (legacy)
}

// UserActor is the concrete actor for an authenticated user.
// Scope is stored as key-value; TenantID/OrganizationID/ProjectID are convenience accessors.
type UserActor struct {
	id          string
	displayName string
	email       string
	subject     string
	clientID    string
	realm       string
	roles       []string
	scopes      []string
	attrs       map[string]string
	metadata    map[string]string
	scope       map[string]string
}

// NewUserActor creates a UserActor from params. All fields are optional except ID.
func NewUserActor(p UserActorParams) *UserActor {
	return &UserActor{
		id:          p.ID,
		displayName: p.DisplayName,
		email:       p.Email,
		subject:     p.Subject,
		clientID:    p.ClientID,
		realm:       p.Realm,
		roles:       p.Roles,
		scopes:      p.Scopes,
		attrs:       p.Attrs,
		metadata:    p.Metadata,
		scope:       make(map[string]string),
	}
}

func (u *UserActor) ID() string          { return u.id }
func (u *UserActor) Type() Type          { return TypeUser }
func (u *UserActor) DisplayName() string { return u.displayName }
func (u *UserActor) Email() string       { return u.email }
func (u *UserActor) Subject() string     { return u.subject }
func (u *UserActor) ClientID() string    { return u.clientID }
func (u *UserActor) Realm() string       { return u.realm }

func (u *UserActor) Roles() []string {
	if u.roles == nil {
		return []string{}
	}
	return u.roles
}

func (u *UserActor) Scopes() []string {
	if u.scopes == nil {
		return []string{}
	}
	return u.scopes
}

func (u *UserActor) Attrs() map[string]string {
	if u.attrs == nil {
		return map[string]string{}
	}
	return u.attrs
}

func (u *UserActor) Scope(key string) string {
	if u.scope == nil {
		return ""
	}
	return u.scope[key]
}

func (u *UserActor) SetScope(key, value string) {
	if u.scope == nil {
		u.scope = make(map[string]string)
	}
	u.scope[key] = value
}

func (u *UserActor) Metadata() map[string]string {
	if u.metadata == nil {
		return map[string]string{}
	}
	return u.metadata
}

func (u *UserActor) Meta(key string) string {
	if u.metadata == nil {
		return ""
	}
	return u.metadata[key]
}

func (u *UserActor) TenantID() string            { return u.Scope(ScopeKeyTenantID) }
func (u *UserActor) OrganizationID() string      { return u.Scope(ScopeKeyOrganizationID) }
func (u *UserActor) ProjectID() string           { return u.Scope(ScopeKeyProjectID) }
func (u *UserActor) SetTenantID(id string)       { u.SetScope(ScopeKeyTenantID, id) }
func (u *UserActor) SetOrganizationID(id string) { u.SetScope(ScopeKeyOrganizationID, id) }
func (u *UserActor) SetProjectID(id string)      { u.SetScope(ScopeKeyProjectID, id) }
