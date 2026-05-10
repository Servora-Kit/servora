package actor

// AnonymousActor represents an unauthenticated request initiator.
type AnonymousActor struct{}

func NewAnonymousActor() *AnonymousActor { return &AnonymousActor{} }

func (a *AnonymousActor) ID() string          { return "anonymous" }
func (a *AnonymousActor) Type() Type          { return TypeAnonymous }
func (a *AnonymousActor) DisplayName() string { return "anonymous" }
