package openfga

import (
	"context"
	"fmt"

	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/obs/audit"
	fgaclient "github.com/openfga/go-sdk/client"
)

// Tuple represents a single OpenFGA relationship tuple.
type Tuple struct {
	User     string // e.g. "user:uuid" or "organization:uuid"
	Relation string // e.g. "owner", "admin", "tenant"
	Object   string // e.g. "organization:uuid", "project:uuid"
}

// WriteTuples writes one or more relationship tuples atomically and emits
// an audit event on success when a recorder is configured.
func (c *Client) WriteTuples(ctx context.Context, tuples ...Tuple) error {
	if err := c.writeTuplesCore(ctx, tuples...); err != nil {
		return err
	}
	c.emitTupleAudit(ctx, "openfga.WriteTuples", audit.TupleMutationWrite, tuples)
	return nil
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

// DeleteTuples deletes one or more relationship tuples atomically and emits
// an audit event on success when a recorder is configured.
func (c *Client) DeleteTuples(ctx context.Context, tuples ...Tuple) error {
	if err := c.deleteTuplesCore(ctx, tuples...); err != nil {
		return err
	}
	c.emitTupleAudit(ctx, "openfga.DeleteTuples", audit.TupleMutationDelete, tuples)
	return nil
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
	return nil
}

func (c *Client) emitTupleAudit(ctx context.Context, operation string, mutation audit.TupleMutationType, tuples []Tuple) {
	if c.recorder == nil || len(tuples) == 0 {
		return
	}
	changes := make([]audit.TupleChange, len(tuples))
	for i, t := range tuples {
		changes[i] = audit.TupleChange{User: t.User, Relation: t.Relation, Object: t.Object}
	}
	a, _ := actor.FromContext(ctx)
	c.recorder.RecordTupleChange(ctx, operation, a, audit.TupleMutationDetail{
		MutationType: mutation,
		Tuples:       changes,
	})
}
