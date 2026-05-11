package apikey

import "context"

// SubjectFrom extracts the owner subject identifier from ctx. Returns
// the OwnerID previously written by the apikey authenticator on success,
// or ("", false) if no [KeyMeta] is present or the OwnerID is empty.
func SubjectFrom(ctx context.Context) (string, bool) {
	meta, ok := KeyMetaFrom(ctx)
	if !ok || meta.OwnerID == "" {
		return "", false
	}
	return meta.OwnerID, true
}
