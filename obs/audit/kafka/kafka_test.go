package kafka

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Servora-Kit/servora/obs/audit"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/twmb/franz-go/pkg/kgo"
)

func TestEncodeRecordBinaryMode(t *testing.T) {
	ev := testEvent(t)
	record, err := EncodeRecord("audit-topic", ev, BinaryMode, nil)
	if err != nil {
		t.Fatalf("EncodeRecord() error = %v", err)
	}

	if record.Topic != "audit-topic" {
		t.Fatalf("Topic = %q, want audit-topic", record.Topic)
	}
	if string(record.Key) != "tenant-1" {
		t.Fatalf("Key = %q, want tenant-1", record.Key)
	}
	if string(record.Value) != `{"ok":true}` {
		t.Fatalf("Value = %q, want raw event data", record.Value)
	}
	if got := header(record.Headers, "ce_id"); got != "event-1" {
		t.Fatalf("ce_id = %q, want event-1", got)
	}
	if got := header(record.Headers, "ce_type"); got != "servora.audit.test" {
		t.Fatalf("ce_type = %q, want servora.audit.test", got)
	}
	if got := header(record.Headers, ContentTypeHeader); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	if got := header(record.Headers, "ce_partitionkey"); got != "tenant-1" {
		t.Fatalf("ce_partitionkey = %q, want tenant-1", got)
	}
}

func TestEncodeRecordStructuredJSONMode(t *testing.T) {
	record, err := EncodeRecord("audit-topic", testEvent(t), StructuredJSONMode, nil)
	if err != nil {
		t.Fatalf("EncodeRecord() error = %v", err)
	}

	if string(record.Key) != "tenant-1" {
		t.Fatalf("Key = %q, want tenant-1", record.Key)
	}
	if got := header(record.Headers, ContentTypeHeader); got != CloudEventsJSONContentType {
		t.Fatalf("content-type = %q, want %s", got, CloudEventsJSONContentType)
	}

	ev, err := DecodeRecord(record)
	if err != nil {
		t.Fatalf("DecodeRecord() error = %v", err)
	}
	if ev.ID() != "event-1" {
		t.Fatalf("decoded ID = %q, want event-1", ev.ID())
	}
}

func TestDecodeRecordBinaryMode(t *testing.T) {
	record, err := EncodeRecord("audit-topic", testEvent(t), BinaryMode, nil)
	if err != nil {
		t.Fatalf("EncodeRecord() error = %v", err)
	}

	ev, err := DecodeRecord(record)
	if err != nil {
		t.Fatalf("DecodeRecord() error = %v", err)
	}
	if ev.ID() != "event-1" || ev.Type() != "servora.audit.test" || ev.Source() != "/test.Service/Method" {
		t.Fatalf("decoded event context = id:%q type:%q source:%q", ev.ID(), ev.Type(), ev.Source())
	}
	if got := ev.Extensions()[audit.ExtPartitionKey]; got != "tenant-1" {
		t.Fatalf("partitionkey extension = %v, want tenant-1", got)
	}
}

func TestDefaultPartitionKeyFallsBackToEventID(t *testing.T) {
	ev := testEvent(t)
	ev.SetExtension(audit.ExtPartitionKey, "")
	if got := DefaultPartitionKey(ev); got != "event-1" {
		t.Fatalf("DefaultPartitionKey() = %q, want event-1", got)
	}
}

func TestAuditorEmitFlushClose(t *testing.T) {
	fp := &fakeProducer{}
	aud, err := newAuditorWithProducer(Config{Topic: "audit-topic"}, fp, testLogger())
	if err != nil {
		t.Fatalf("newAuditorWithProducer() error = %v", err)
	}
	if err := aud.Emit(context.Background(), testEvent(t)); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}
	if len(fp.records) != 1 {
		t.Fatalf("produced records = %d, want 1", len(fp.records))
	}
	if err := aud.(audit.Flusher).Flush(context.Background()); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if err := aud.(audit.Closer).Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !fp.flushed || !fp.closed {
		t.Fatalf("flushed=%v closed=%v, want true true", fp.flushed, fp.closed)
	}
}

func TestAuditorEmitReturnsProduceError(t *testing.T) {
	want := errors.New("produce failed")
	fp := &fakeProducer{err: want}
	aud, err := newAuditorWithProducer(Config{}, fp, testLogger())
	if err != nil {
		t.Fatalf("newAuditorWithProducer() error = %v", err)
	}
	if err := aud.Emit(context.Background(), testEvent(t)); !errors.Is(err, want) {
		t.Fatalf("Emit() error = %v, want %v", err, want)
	}
}

type fakeProducer struct {
	records []*kgo.Record
	err     error
	flushed bool
	closed  bool
}

func (f *fakeProducer) ProduceSync(_ context.Context, records ...*kgo.Record) kgo.ProduceResults {
	f.records = append(f.records, records...)
	results := make(kgo.ProduceResults, 0, len(records))
	for _, record := range records {
		results = append(results, kgo.ProduceResult{Record: record, Err: f.err})
	}
	return results
}

func (f *fakeProducer) Flush(context.Context) error {
	f.flushed = true
	return nil
}

func (f *fakeProducer) Close() {
	f.closed = true
}

func testEvent(t *testing.T) cloudevents.Event {
	t.Helper()
	ev := cloudevents.NewEvent()
	ev.SetID("event-1")
	ev.SetType("servora.audit.test")
	ev.SetSource("/test.Service/Method")
	ev.SetTime(time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))
	ev.SetSubject("target-1")
	ev.SetExtension(audit.ExtPartitionKey, "tenant-1")
	if err := ev.SetData("application/json", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("SetData() error = %v", err)
	}
	return ev
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
