package actor

type SystemActor struct {
	id          string
	serviceName string
}

// NewSystemActor creates a SystemActor. id is the fully-qualified OpenFGA principal
// (e.g. "system:my-service"). serviceName is the human-readable service name.
func NewSystemActor(id, serviceName string) *SystemActor {
	return &SystemActor{id: id, serviceName: serviceName}
}

func (s *SystemActor) ID() string                  { return s.id }
func (s *SystemActor) Type() Type                  { return TypeSystem }
func (s *SystemActor) DisplayName() string         { return s.serviceName }
func (s *SystemActor) ServiceName() string         { return s.serviceName }
func (s *SystemActor) Email() string               { return "" }
func (s *SystemActor) Subject() string             { return "" }
func (s *SystemActor) ClientID() string            { return "" }
func (s *SystemActor) Realm() string               { return "" }
func (s *SystemActor) Roles() []string             { return []string{} }
func (s *SystemActor) Scopes() []string            { return []string{} }
func (s *SystemActor) Attrs() map[string]string    { return map[string]string{} }
