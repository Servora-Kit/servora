package apikey

// config aggregates optional dependencies of an apikey [authenticator].
// All fields are package-private; callers configure them via [Option]
// constructors.
type config struct {
	store Store
}

// Option configures an apikey [authenticator] at construction time.
type Option func(*config)

// WithStore configures the storage backend used to resolve API keys into
// [KeyMeta]. It is REQUIRED — [NewAuthenticator] panics if called without
// `WithStore` (fail-fast: an apikey engine without a Store can never
// authenticate any request, so silent acceptance would mask wiring bugs).
func WithStore(s Store) Option {
	return func(c *config) { c.store = s }
}
