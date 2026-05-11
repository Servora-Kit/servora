package cev1

import (
	"bytes"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestCloudEvent_BinaryData_Roundtrip(t *testing.T) {
	t.Parallel()

	now := timestamppb.New(time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC))

	original := &CloudEvent{
		Id:          "event-001",
		Source:      "//servora/test",
		SpecVersion: "1.0",
		Type:        "com.servora.test.v1",
		Attributes: map[string]*CloudEvent_CloudEventAttributeValue{
			"bool_attr": {
				Attr: &CloudEvent_CloudEventAttributeValue_CeBoolean{CeBoolean: true},
			},
			"int_attr": {
				Attr: &CloudEvent_CloudEventAttributeValue_CeInteger{CeInteger: 42},
			},
			"string_attr": {
				Attr: &CloudEvent_CloudEventAttributeValue_CeString{CeString: "hello"},
			},
			"bytes_attr": {
				Attr: &CloudEvent_CloudEventAttributeValue_CeBytes{CeBytes: []byte{0xDE, 0xAD}},
			},
			"uri_attr": {
				Attr: &CloudEvent_CloudEventAttributeValue_CeUri{CeUri: "https://example.com/resource"},
			},
			"uri_ref_attr": {
				Attr: &CloudEvent_CloudEventAttributeValue_CeUriRef{CeUriRef: "/relative/path"},
			},
			"timestamp_attr": {
				Attr: &CloudEvent_CloudEventAttributeValue_CeTimestamp{CeTimestamp: now},
			},
		},
		Data: &CloudEvent_BinaryData{BinaryData: []byte("binary payload")},
	}

	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("proto.Marshal failed: %v", err)
	}

	got := &CloudEvent{}
	if err := proto.Unmarshal(data, got); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}

	// Required fields
	if got.GetId() != original.GetId() {
		t.Errorf("Id: got %q, want %q", got.GetId(), original.GetId())
	}
	if got.GetSource() != original.GetSource() {
		t.Errorf("Source: got %q, want %q", got.GetSource(), original.GetSource())
	}
	if got.GetSpecVersion() != original.GetSpecVersion() {
		t.Errorf("SpecVersion: got %q, want %q", got.GetSpecVersion(), original.GetSpecVersion())
	}
	if got.GetType() != original.GetType() {
		t.Errorf("Type: got %q, want %q", got.GetType(), original.GetType())
	}

	// Data oneof (binary)
	if !bytes.Equal(got.GetBinaryData(), original.GetBinaryData()) {
		t.Errorf("BinaryData: got %x, want %x", got.GetBinaryData(), original.GetBinaryData())
	}

	// Attributes - all 7 oneof variants
	attrs := got.GetAttributes()
	if len(attrs) != 7 {
		t.Fatalf("Attributes count: got %d, want 7", len(attrs))
	}

	if v := attrs["bool_attr"].GetCeBoolean(); v != true {
		t.Errorf("CeBoolean: got %v, want true", v)
	}
	if v := attrs["int_attr"].GetCeInteger(); v != 42 {
		t.Errorf("CeInteger: got %d, want 42", v)
	}
	if v := attrs["string_attr"].GetCeString(); v != "hello" {
		t.Errorf("CeString: got %q, want %q", v, "hello")
	}
	if v := attrs["bytes_attr"].GetCeBytes(); !bytes.Equal(v, []byte{0xDE, 0xAD}) {
		t.Errorf("CeBytes: got %x, want dead", v)
	}
	if v := attrs["uri_attr"].GetCeUri(); v != "https://example.com/resource" {
		t.Errorf("CeUri: got %q, want %q", v, "https://example.com/resource")
	}
	if v := attrs["uri_ref_attr"].GetCeUriRef(); v != "/relative/path" {
		t.Errorf("CeUriRef: got %q, want %q", v, "/relative/path")
	}
	ts := attrs["timestamp_attr"].GetCeTimestamp()
	if ts == nil {
		t.Fatal("CeTimestamp: got nil")
	}
	if ts.GetSeconds() != now.GetSeconds() || ts.GetNanos() != now.GetNanos() {
		t.Errorf("CeTimestamp: got %v, want %v", ts, now)
	}
}

func TestCloudEvent_TextData_Roundtrip(t *testing.T) {
	t.Parallel()

	original := &CloudEvent{
		Id:          "event-002",
		Source:      "//servora/text-test",
		SpecVersion: "1.0",
		Type:        "com.servora.text.v1",
		Data:        &CloudEvent_TextData{TextData: "this is text payload with unicode: 你好世界"},
	}

	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("proto.Marshal failed: %v", err)
	}

	got := &CloudEvent{}
	if err := proto.Unmarshal(data, got); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}

	if got.GetTextData() != original.GetTextData() {
		t.Errorf("TextData: got %q, want %q", got.GetTextData(), original.GetTextData())
	}
	// Ensure binary_data is NOT set
	if got.GetBinaryData() != nil {
		t.Errorf("BinaryData should be nil when TextData is set, got %x", got.GetBinaryData())
	}
}

func TestCloudEvent_ProtoData_Roundtrip(t *testing.T) {
	t.Parallel()

	// Use a timestamppb.Timestamp as the Any payload for simplicity.
	inner := timestamppb.New(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	anyVal, err := anypb.New(inner)
	if err != nil {
		t.Fatalf("anypb.New failed: %v", err)
	}

	original := &CloudEvent{
		Id:          "event-003",
		Source:      "//servora/proto-test",
		SpecVersion: "1.0",
		Type:        "com.servora.proto.v1",
		Data:        &CloudEvent_ProtoData{ProtoData: anyVal},
	}

	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("proto.Marshal failed: %v", err)
	}

	got := &CloudEvent{}
	if err := proto.Unmarshal(data, got); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}

	gotProto := got.GetProtoData()
	if gotProto == nil {
		t.Fatal("ProtoData: got nil")
	}
	if gotProto.GetTypeUrl() != anyVal.GetTypeUrl() {
		t.Errorf("ProtoData.TypeUrl: got %q, want %q", gotProto.GetTypeUrl(), anyVal.GetTypeUrl())
	}
	if !bytes.Equal(gotProto.GetValue(), anyVal.GetValue()) {
		t.Errorf("ProtoData.Value: got %x, want %x", gotProto.GetValue(), anyVal.GetValue())
	}

	// Unpack and verify the inner message
	var decoded timestamppb.Timestamp
	if err := gotProto.UnmarshalTo(&decoded); err != nil {
		t.Fatalf("UnmarshalTo failed: %v", err)
	}
	if decoded.GetSeconds() != inner.GetSeconds() || decoded.GetNanos() != inner.GetNanos() {
		t.Errorf("Decoded inner timestamp: got %v, want %v", &decoded, inner)
	}
}

func TestCloudEventBatch_Roundtrip(t *testing.T) {
	t.Parallel()

	batch := &CloudEventBatch{
		Events: []*CloudEvent{
			{
				Id:          "batch-event-1",
				Source:      "//servora/batch",
				SpecVersion: "1.0",
				Type:        "com.servora.batch.v1",
				Data:        &CloudEvent_TextData{TextData: "first"},
			},
			{
				Id:          "batch-event-2",
				Source:      "//servora/batch",
				SpecVersion: "1.0",
				Type:        "com.servora.batch.v1",
				Data:        &CloudEvent_BinaryData{BinaryData: []byte("second")},
				Attributes: map[string]*CloudEvent_CloudEventAttributeValue{
					"priority": {
						Attr: &CloudEvent_CloudEventAttributeValue_CeInteger{CeInteger: 10},
					},
				},
			},
			{
				Id:          "batch-event-3",
				Source:      "//servora/batch",
				SpecVersion: "1.0",
				Type:        "com.servora.batch.v1",
				Data:        &CloudEvent_TextData{TextData: "third"},
			},
		},
	}

	data, err := proto.Marshal(batch)
	if err != nil {
		t.Fatalf("proto.Marshal failed: %v", err)
	}

	got := &CloudEventBatch{}
	if err := proto.Unmarshal(data, got); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}

	if len(got.GetEvents()) != 3 {
		t.Fatalf("Events count: got %d, want 3", len(got.GetEvents()))
	}

	// Verify each event's id
	wantIDs := []string{"batch-event-1", "batch-event-2", "batch-event-3"}
	for i, ev := range got.GetEvents() {
		if ev.GetId() != wantIDs[i] {
			t.Errorf("Event[%d].Id: got %q, want %q", i, ev.GetId(), wantIDs[i])
		}
	}

	// Verify second event's data and attribute
	ev2 := got.GetEvents()[1]
	if !bytes.Equal(ev2.GetBinaryData(), []byte("second")) {
		t.Errorf("Event[1].BinaryData: got %x, want %x", ev2.GetBinaryData(), []byte("second"))
	}
	if ev2.GetAttributes()["priority"].GetCeInteger() != 10 {
		t.Errorf("Event[1].Attributes[priority]: got %d, want 10",
			ev2.GetAttributes()["priority"].GetCeInteger())
	}
}

func TestCloudEventAttributeValue_AllVariants(t *testing.T) {
	t.Parallel()

	now := timestamppb.Now()

	tests := []struct {
		name   string
		attr   *CloudEvent_CloudEventAttributeValue
		verify func(t *testing.T, got *CloudEvent_CloudEventAttributeValue)
	}{
		{
			name: "CeBoolean",
			attr: &CloudEvent_CloudEventAttributeValue{
				Attr: &CloudEvent_CloudEventAttributeValue_CeBoolean{CeBoolean: true},
			},
			verify: func(t *testing.T, got *CloudEvent_CloudEventAttributeValue) {
				if got.GetCeBoolean() != true {
					t.Errorf("got %v, want true", got.GetCeBoolean())
				}
			},
		},
		{
			name: "CeInteger",
			attr: &CloudEvent_CloudEventAttributeValue{
				Attr: &CloudEvent_CloudEventAttributeValue_CeInteger{CeInteger: -999},
			},
			verify: func(t *testing.T, got *CloudEvent_CloudEventAttributeValue) {
				if got.GetCeInteger() != -999 {
					t.Errorf("got %d, want -999", got.GetCeInteger())
				}
			},
		},
		{
			name: "CeString",
			attr: &CloudEvent_CloudEventAttributeValue{
				Attr: &CloudEvent_CloudEventAttributeValue_CeString{CeString: "test-value"},
			},
			verify: func(t *testing.T, got *CloudEvent_CloudEventAttributeValue) {
				if got.GetCeString() != "test-value" {
					t.Errorf("got %q, want %q", got.GetCeString(), "test-value")
				}
			},
		},
		{
			name: "CeBytes",
			attr: &CloudEvent_CloudEventAttributeValue{
				Attr: &CloudEvent_CloudEventAttributeValue_CeBytes{CeBytes: []byte{0x01, 0x02, 0x03}},
			},
			verify: func(t *testing.T, got *CloudEvent_CloudEventAttributeValue) {
				if !bytes.Equal(got.GetCeBytes(), []byte{0x01, 0x02, 0x03}) {
					t.Errorf("got %x, want 010203", got.GetCeBytes())
				}
			},
		},
		{
			name: "CeUri",
			attr: &CloudEvent_CloudEventAttributeValue{
				Attr: &CloudEvent_CloudEventAttributeValue_CeUri{CeUri: "https://example.com"},
			},
			verify: func(t *testing.T, got *CloudEvent_CloudEventAttributeValue) {
				if got.GetCeUri() != "https://example.com" {
					t.Errorf("got %q, want %q", got.GetCeUri(), "https://example.com")
				}
			},
		},
		{
			name: "CeUriRef",
			attr: &CloudEvent_CloudEventAttributeValue{
				Attr: &CloudEvent_CloudEventAttributeValue_CeUriRef{CeUriRef: "/path/to/thing"},
			},
			verify: func(t *testing.T, got *CloudEvent_CloudEventAttributeValue) {
				if got.GetCeUriRef() != "/path/to/thing" {
					t.Errorf("got %q, want %q", got.GetCeUriRef(), "/path/to/thing")
				}
			},
		},
		{
			name: "CeTimestamp",
			attr: &CloudEvent_CloudEventAttributeValue{
				Attr: &CloudEvent_CloudEventAttributeValue_CeTimestamp{CeTimestamp: now},
			},
			verify: func(t *testing.T, got *CloudEvent_CloudEventAttributeValue) {
				ts := got.GetCeTimestamp()
				if ts == nil {
					t.Fatal("got nil timestamp")
				}
				if ts.GetSeconds() != now.GetSeconds() || ts.GetNanos() != now.GetNanos() {
					t.Errorf("got %v, want %v", ts, now)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Wrap in a CloudEvent to test as part of the map
			original := &CloudEvent{
				Id:          "attr-test",
				Source:      "//test",
				SpecVersion: "1.0",
				Type:        "test.attr",
				Attributes: map[string]*CloudEvent_CloudEventAttributeValue{
					"test_key": tt.attr,
				},
				Data: &CloudEvent_BinaryData{BinaryData: []byte("x")},
			}

			data, err := proto.Marshal(original)
			if err != nil {
				t.Fatalf("proto.Marshal failed: %v", err)
			}

			got := &CloudEvent{}
			if err := proto.Unmarshal(data, got); err != nil {
				t.Fatalf("proto.Unmarshal failed: %v", err)
			}

			attr, ok := got.GetAttributes()["test_key"]
			if !ok {
				t.Fatal("missing attribute 'test_key'")
			}
			tt.verify(t, attr)
		})
	}
}
