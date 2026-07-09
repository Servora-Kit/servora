package openfga

import (
	"context"
	"fmt"

	openfgaauditpb "github.com/Servora-Kit/servora/api/gen/go/servora/authz/openfga/audit/v1"
	"github.com/Servora-Kit/servora/obs/audit"
	fgaclient "github.com/openfga/go-sdk/client"
)

// EventTypeOpenFGATupleMutation is the CloudEvents type for OpenFGA tuple write/delete events.
const EventTypeOpenFGATupleMutation = "servora.authz.openfga.tuple_mutation.v1"

// Tuple represents a single OpenFGA relationship tuple.
type Tuple struct {
	User     string // e.g. "user:uuid" or "organization:uuid"
	Relation string // e.g. "owner", "admin", "tenant"
	Object   string // e.g. "organization:uuid", "project:uuid"
}

// WriteTuples writes one or more relationship tuples atomically.
func (c *Client) WriteTuples(ctx context.Context, tuples ...Tuple) error {
	return c.writeTuplesCore(ctx, tuples...)
}

func (c *Client) writeTuplesCore(ctx context.Context, tuples ...Tuple) error {
	if len(tuples) == 0 {
		return nil
	}
	writes := make([]fgaclient.ClientTupleKey, len(tuples))
	for i, t := range tuples {
		writes[i] = fgaclient.ClientTupleKey{
			User:     t.User,
			Relation: t.Relation,
			Object:   t.Object,
		}
	}
	_, err := c.sdk.Write(ctx).
		Body(fgaclient.ClientWriteRequest{Writes: writes}).
		Execute()
	if err != nil {
		return fmt.Errorf("openfga write: %w", err)
	}
	// Emit audit event on success only.
	c.emitTupleMutation(ctx, openfgaauditpb.TupleMutation_OPERATION_WRITE, tuples)
	return nil
}

// TupleExists reports whether the exact tuple already exists in the store.
func (c *Client) TupleExists(ctx context.Context, t Tuple) (bool, error) {
	resp, err := c.sdk.Read(ctx).
		Body(fgaclient.ClientReadRequest{
			User:     &t.User,
			Relation: &t.Relation,
			Object:   &t.Object,
		}).
		Execute()
	if err != nil {
		return false, fmt.Errorf("openfga read: %w", err)
	}
	return len(resp.GetTuples()) > 0, nil
}

// EnsureTuples writes each tuple only if it does not already exist.
// It is safe to call repeatedly (idempotent) and does not rely on error
// message text matching.
func (c *Client) EnsureTuples(ctx context.Context, tuples ...Tuple) error {
	for _, t := range tuples {
		exists, err := c.TupleExists(ctx, t)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if err := c.WriteTuples(ctx, t); err != nil {
			return err
		}
	}
	return nil
}

// DeleteTuples deletes one or more relationship tuples atomically.
func (c *Client) DeleteTuples(ctx context.Context, tuples ...Tuple) error {
	return c.deleteTuplesCore(ctx, tuples...)
}

func (c *Client) deleteTuplesCore(ctx context.Context, tuples ...Tuple) error {
	if len(tuples) == 0 {
		return nil
	}
	deletes := make([]fgaclient.ClientTupleKeyWithoutCondition, len(tuples))
	for i, t := range tuples {
		deletes[i] = fgaclient.ClientTupleKeyWithoutCondition{
			User:     t.User,
			Relation: t.Relation,
			Object:   t.Object,
		}
	}
	_, err := c.sdk.Write(ctx).
		Body(fgaclient.ClientWriteRequest{Deletes: deletes}).
		Execute()
	if err != nil {
		return fmt.Errorf("openfga delete: %w", err)
	}
	// Emit audit event on success only.
	c.emitTupleMutation(ctx, openfgaauditpb.TupleMutation_OPERATION_DELETE, tuples)
	return nil
}

// emitTupleMutation emits a CloudEvents audit event for a tuple write or delete.
// Best-effort: errors are silently ignored. No-op when auditor is not configured.
func (c *Client) emitTupleMutation(ctx context.Context, op openfgaauditpb.TupleMutation_Operation, tuples []Tuple) {
	if c.auditor == nil {
		return
	}
	pbTuples := make([]*openfgaauditpb.Tuple, len(tuples))
	for i, t := range tuples {
		pbTuples[i] = &openfgaauditpb.Tuple{
			User:     t.User,
			Relation: t.Relation,
			Object:   t.Object,
		}
	}
	subject := "openfga/store/" + c.storeID
	event := audit.NewEvent(ctx,
		audit.WithType(EventTypeOpenFGATupleMutation),
		audit.WithSubject(subject),
	)
	data := &openfgaauditpb.TupleMutation{
		Operation: op,
		Tuples:    pbTuples,
		StoreId:   c.storeID,
	}
	_ = audit.SetProtoData(&event, data)
	_ = c.auditor.Emit(ctx, event)
}
