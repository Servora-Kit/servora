package openfga

import (
	"context"
	"errors"
	"testing"

	fgaclient "github.com/openfga/go-sdk/client"
)

func TestBuildBatchCheckItems_PreservesOrderAndMapping(t *testing.T) {
	reqs := []BatchCheckItem{
		{User: "user:alice", Relation: "viewer", Object: "doc:1"},
		{User: "user:bob", Relation: "editor", Object: "doc:2"},
		{User: "user:carol", Relation: "owner", Object: "doc:3"},
	}

	items := buildBatchCheckItems(reqs)

	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}
	for i, item := range items {
		if item.User != reqs[i].User {
			t.Errorf("item[%d].User = %q, want %q", i, item.User, reqs[i].User)
		}
		if item.Relation != reqs[i].Relation {
			t.Errorf("item[%d].Relation = %q, want %q", i, item.Relation, reqs[i].Relation)
		}
		if item.Object != reqs[i].Object {
			t.Errorf("item[%d].Object = %q, want %q", i, item.Object, reqs[i].Object)
		}
		// CorrelationId must be the index — used to map response back.
		wantCorr := correlationIDFromIndex(i)
		if item.CorrelationId != wantCorr {
			t.Errorf("item[%d].CorrelationId = %q, want %q", i, item.CorrelationId, wantCorr)
		}
	}

	// Ensure the helper actually emits ClientBatchCheckItem (compile-time check).
	_ = []fgaclient.ClientBatchCheckItem(items)
}

func TestMapBatchCheckResults_BackToOrderedResults(t *testing.T) {
	allowed := map[string]bool{
		correlationIDFromIndex(0): true,
		correlationIDFromIndex(1): false,
		correlationIDFromIndex(2): true,
	}
	errs := map[string]error{
		correlationIDFromIndex(1): errors.New("backend boom"),
	}

	out := mapBatchCheckResults(3, allowed, errs)

	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3", len(out))
	}
	if !out[0].Allowed || out[1].Allowed || !out[2].Allowed {
		t.Errorf("allowed pattern = [%v %v %v], want [true false true]",
			out[0].Allowed, out[1].Allowed, out[2].Allowed)
	}
	if out[1].Err == nil {
		t.Errorf("out[1].Err = nil, want non-nil")
	}
	if out[0].Err != nil || out[2].Err != nil {
		t.Errorf("out[0].Err=%v out[2].Err=%v, both want nil", out[0].Err, out[2].Err)
	}
}

func TestBatchCheck_EmptyInput_ReturnsNilNil(t *testing.T) {
	c := &Client{}
	got, err := c.BatchCheck(context.Background(), nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}
