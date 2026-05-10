package actor

// SystemActor represents a non-user, non-service framework / platform principal
// (e.g. cron jobs, internal background processes).
type SystemActor struct {
	id          string
	displayName string
}

// NewSystemActor creates a SystemActor. id is the fully-qualified principal
// (e.g. "system:my-service"). serviceName 即 DisplayName 语义。
func NewSystemActor(id, serviceName string) *SystemActor {
	return &SystemActor{id: id, displayName: serviceName}
}

func (s *SystemActor) ID() string          { return s.id }
func (s *SystemActor) Type() Type          { return TypeSystem }
func (s *SystemActor) DisplayName() string { return s.displayName }
