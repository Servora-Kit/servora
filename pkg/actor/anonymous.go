package actor

type AnonymousActor struct{}

func NewAnonymousActor() *AnonymousActor { return &AnonymousActor{} }

func (a *AnonymousActor) ID() string                  { return "anonymous" }
func (a *AnonymousActor) Type() Type                  { return TypeAnonymous }
func (a *AnonymousActor) DisplayName() string         { return "anonymous" }
func (a *AnonymousActor) Email() string               { return "" }
func (a *AnonymousActor) Subject() string             { return "" }
func (a *AnonymousActor) ClientID() string            { return "" }
func (a *AnonymousActor) Realm() string               { return "" }
func (a *AnonymousActor) Roles() []string             { return []string{} }
func (a *AnonymousActor) Scopes() []string            { return []string{} }
func (a *AnonymousActor) Attrs() map[string]string    { return map[string]string{} }
func (a *AnonymousActor) Scope(_ string) string       { return "" }
