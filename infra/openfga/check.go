package openfga

import (
	"context"
	"fmt"

	fgaclient "github.com/openfga/go-sdk/client"
)

// Check returns whether the given principal (e.g. "user:uuid") has the specified
// relation on objectType:objectID.
func (c *Client) Check(ctx context.Context, user, relation, objectType, objectID string) (bool, error) {
	resp, err := c.sdk.Check(ctx).
		Body(fgaclient.ClientCheckRequest{
			User:     user,
			Relation: relation,
			Object:   objectType + ":" + objectID,
		}).
		Execute()
	if err != nil {
		return false, fmt.Errorf("openfga check: %w", err)
	}
	return resp.GetAllowed(), nil
}
