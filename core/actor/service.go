package actor

// ServiceActor represents a service-to-service caller identity (machine principal).
// It is used when X-Principal-Type: service is injected by the gateway.
type ServiceActor struct {
	id          string
	displayName string
}

// NewServiceActor creates a ServiceActor with the canonical three-piece identity.
func NewServiceActor(id, displayName string) *ServiceActor {
	return &ServiceActor{
		id:          id,
		displayName: displayName,
	}
}

func (s *ServiceActor) ID() string          { return s.id }
func (s *ServiceActor) Type() Type          { return TypeService }
func (s *ServiceActor) DisplayName() string { return s.displayName }
