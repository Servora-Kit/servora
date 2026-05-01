package openfga

import (
	"context"
	"fmt"
	"strconv"

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

// BatchCheckItem is one element in a BatchCheck request.
// It mirrors fgaclient.ClientBatchCheckItem but is part of our stable API.
type BatchCheckItem struct {
	User     string
	Relation string
	Object   string
}

// BatchCheckResult is the per-item outcome from BatchCheck.
// Order matches the input slice index.
type BatchCheckResult struct {
	Allowed bool
	Err     error
}

// BatchCheck runs N checks in one OpenFGA call. Output order matches input order.
// Returns a top-level error only if the whole call fails; per-item errors land
// in BatchCheckResult.Err for that item.
func (c *Client) BatchCheck(ctx context.Context, reqs []BatchCheckItem) ([]BatchCheckResult, error) {
	if len(reqs) == 0 {
		return nil, nil
	}

	items := buildBatchCheckItems(reqs)

	resp, err := c.sdk.BatchCheck(ctx).
		Body(fgaclient.ClientBatchCheckRequest{Checks: items}).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("openfga batch check: %w", err)
	}

	allowed := make(map[string]bool, len(reqs))
	errs := make(map[string]error)
	for corr, single := range resp.GetResult() {
		allowed[corr] = single.GetAllowed()
		if e := single.Error; e != nil {
			errs[corr] = fmt.Errorf("openfga batch item %s: %s", corr, e.GetMessage())
		}
	}

	return mapBatchCheckResults(len(reqs), allowed, errs), nil
}

func buildBatchCheckItems(reqs []BatchCheckItem) []fgaclient.ClientBatchCheckItem {
	items := make([]fgaclient.ClientBatchCheckItem, len(reqs))
	for i, r := range reqs {
		items[i] = fgaclient.ClientBatchCheckItem{
			User:          r.User,
			Relation:      r.Relation,
			Object:        r.Object,
			CorrelationId: correlationIDFromIndex(i),
		}
	}
	return items
}

func mapBatchCheckResults(n int, allowed map[string]bool, errs map[string]error) []BatchCheckResult {
	out := make([]BatchCheckResult, n)
	for i := range n {
		corr := correlationIDFromIndex(i)
		out[i] = BatchCheckResult{
			Allowed: allowed[corr],
			Err:     errs[corr],
		}
	}
	return out
}

func correlationIDFromIndex(i int) string {
	return strconv.Itoa(i)
}
