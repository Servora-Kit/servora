package apikey

import (
	"context"

	"github.com/go-kratos/kratos/v2/transport"
)

// headerName is the inbound HTTP/gRPC metadata header consulted by the
// engine. Deliberately distinct from `Authorization` so the apikey engine
// can coexist with the jwt engine in the same `authn.Multi` decorator
// without header collision.
const headerName = "X-API-Key"

// extractAPIKey returns the X-API-Key header value from the Kratos server
// transport attached to ctx. Returns "" when:
//
//   - ctx carries no server transport (e.g. unit tests calling
//     `Authenticate(context.Background())` directly), or
//   - the header is absent / empty.
//
// Package-private. Business code MUST NOT call this directly; the only
// supported way to feed a key into the engine is by setting the
// `X-API-Key` header on the inbound request.
func extractAPIKey(ctx context.Context) string {
	tr, ok := transport.FromServerContext(ctx)
	if !ok || tr == nil {
		return ""
	}
	return tr.RequestHeader().Get(headerName)
}
