package ent

import (
	"context"
	"time"

	"entgo.io/ent/dialect"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// tracingDriver wraps a dialect.Driver and emits zap logs with OTel trace
// context for every Query/Exec/Tx call.
type tracingDriver struct {
	inner dialect.Driver
	log   *zap.Logger
}

// wrapWithTracing decorates inner with trace-correlated SQL logging.
func wrapWithTracing(inner dialect.Driver, log *zap.Logger) dialect.Driver {
	if log == nil {
		log = zap.NewNop()
	}
	return &tracingDriver{inner: inner, log: log}
}

func (d *tracingDriver) Query(ctx context.Context, query string, args, v any) error {
	start := time.Now()
	err := d.inner.Query(ctx, query, args, v)
	d.logCall(ctx, "ent.query", query, time.Since(start), err)
	return err
}

func (d *tracingDriver) logCall(ctx context.Context, op, sql string, elapsed time.Duration, err error) {
	logCallTo(d.log, ctx, op, sql, elapsed, err)
}

// logCallTo emits a single structured log line with trace correlation.
// Shared by both tracingDriver and tracingTx.
func logCallTo(zlog *zap.Logger, ctx context.Context, op, sql string, elapsed time.Duration, err error) {
	fields := make([]zapcore.Field, 0, 6)
	if sql != "" {
		fields = append(fields, zap.String("sql", sql))
	}
	if elapsed > 0 {
		fields = append(fields, zap.Duration("elapsed", elapsed))
	}
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		fields = append(fields,
			zap.String("trace_id", sc.TraceID().String()),
			zap.String("span_id", sc.SpanID().String()),
		)
	}
	if err != nil {
		fields = append(fields, zap.Error(err))
		zlog.Error(op, fields...)
		return
	}
	zlog.Debug(op, fields...)
}

func (d *tracingDriver) Exec(ctx context.Context, query string, args, v any) error {
	start := time.Now()
	err := d.inner.Exec(ctx, query, args, v)
	d.logCall(ctx, "ent.exec", query, time.Since(start), err)
	return err
}

func (d *tracingDriver) Tx(ctx context.Context) (dialect.Tx, error) {
	tx, err := d.inner.Tx(ctx)
	logCallTo(d.log, ctx, "ent.tx.begin", "", 0, err)
	if err != nil {
		return nil, err
	}
	return &tracingTx{inner: tx, log: d.log, ctx: ctx}, nil
}

func (d *tracingDriver) Close() error    { return d.inner.Close() }
func (d *tracingDriver) Dialect() string { return d.inner.Dialect() }

// tracingTx wraps a dialect.Tx and forwards trace context to per-call logs.
// ctx captured at Begin time is reused for Commit/Rollback log correlation.
type tracingTx struct {
	inner dialect.Tx
	log   *zap.Logger
	ctx   context.Context
}

func (t *tracingTx) Query(ctx context.Context, query string, args, v any) error {
	start := time.Now()
	err := t.inner.Query(ctx, query, args, v)
	logCallTo(t.log, ctx, "ent.tx.query", query, time.Since(start), err)
	return err
}

func (t *tracingTx) Exec(ctx context.Context, query string, args, v any) error {
	start := time.Now()
	err := t.inner.Exec(ctx, query, args, v)
	logCallTo(t.log, ctx, "ent.tx.exec", query, time.Since(start), err)
	return err
}

func (t *tracingTx) Commit() error {
	err := t.inner.Commit()
	logCallTo(t.log, t.ctx, "ent.tx.commit", "", 0, err)
	return err
}

func (t *tracingTx) Rollback() error {
	err := t.inner.Rollback()
	logCallTo(t.log, t.ctx, "ent.tx.rollback", "", 0, err)
	return err
}
