package actor

// ServiceActor represents a service-to-service caller identity (machine principal).
// It is used when X-Principal-Type: service is injected by the gateway.
type ServiceActor struct {
	id          string
	clientID    string
	displayName string
	realm       string
	scopes      []string
	attrs       map[string]string
}

// NewServiceActor creates a ServiceActor.
func NewServiceActor(id, clientID, displayName string) *ServiceActor {
	return &ServiceActor{
		id:          id,
		clientID:    clientID,
		displayName: displayName,
	}
}

func (s *ServiceActor) ID() string          { return s.id }
func (s *ServiceActor) Type() Type          { return TypeService }
func (s *ServiceActor) DisplayName() string { return s.displayName }
func (s *ServiceActor) Email() string       { return "" }
func (s *ServiceActor) Subject() string     { return s.id }
func (s *ServiceActor) ClientID() string    { return s.clientID }
func (s *ServiceActor) Realm() string       { return s.realm }

func (s *ServiceActor) Roles() []string { return []string{} }

func (s *ServiceActor) Scopes() []string {
	if s.scopes == nil {
		return []string{}
	}
	return s.scopes
}

func (s *ServiceActor) Attrs() map[string]string {
	if s.attrs == nil {
		return map[string]string{}
	}
	return s.attrs
}

func (s *ServiceActor) SetRealm(realm string)    { s.realm = realm }
func (s *ServiceActor) SetScopes(scopes []string) { s.scopes = scopes }
