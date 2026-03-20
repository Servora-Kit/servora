package actor

type SystemActor struct {
	serviceName string
}

func NewSystemActor(serviceName string) *SystemActor {
	return &SystemActor{serviceName: serviceName}
}

func (s *SystemActor) ID() string                  { return "system:" + s.serviceName }
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
func (s *SystemActor) Scope(_ string) string       { return "" }
