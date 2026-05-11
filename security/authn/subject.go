package authn

import "context"

// SubjectFromAny composes multiple subject-extraction functions into a single
// combinator. The first function that returns (subject, true) wins; nil
// entries are silently skipped.
//
// This allows business code to combine different identity sources (e.g.
// actor-from-JWT, actor-from-apikey, actor-from-mTLS) into a unified
// subject resolver without coupling to any specific engine.
func SubjectFromAny(fns ...func(context.Context) (string, bool)) func(context.Context) (string, bool) {
	return func(ctx context.Context) (string, bool) {
		for _, fn := range fns {
			if fn == nil {
				continue
			}
			if s, ok := fn(ctx); ok {
				return s, true
			}
		}
		return "", false
	}
}
