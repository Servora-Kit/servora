package actor

// UserActor is the concrete actor for an authenticated user.
type UserActor struct {
	id          string
	displayName string
}

// NewUserActor creates a UserActor with the canonical three-piece identity.
func NewUserActor(id, displayName string) *UserActor {
	return &UserActor{
		id:          id,
		displayName: displayName,
	}
}

func (u *UserActor) ID() string          { return u.id }
func (u *UserActor) Type() Type          { return TypeUser }
func (u *UserActor) DisplayName() string { return u.displayName }
