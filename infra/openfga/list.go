package openfga

import (
	"context"
	"fmt"
	"strings"

	fgaclient "github.com/openfga/go-sdk/client"
)

// ListObjects returns the IDs of objects of the given type that the principal
// (e.g. "user:uuid") has the specified relation to. The returned strings are
// bare IDs (i.e. the "type:" prefix is stripped).
func (c *Client) ListObjects(ctx context.Context, user, relation, objectType string) ([]string, error) {
	resp, err := c.sdk.ListObjects(ctx).
		Body(fgaclient.ClientListObjectsRequest{
			User:     user,
			Relation: relation,
			Type:     objectType,
		}).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("openfga list objects: %w", err)
	}

	prefix := objectType + ":"
	ids := make([]string, 0, len(resp.GetObjects()))
	for _, obj := range resp.GetObjects() {
		ids = append(ids, strings.TrimPrefix(obj, prefix))
	}
	return ids, nil
}
