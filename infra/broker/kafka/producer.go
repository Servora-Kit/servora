package kafka

import (
	"github.com/Servora-Kit/servora/infra/broker"
	"github.com/twmb/franz-go/pkg/kgo"
)

// buildRecord converts a broker.Message to a kgo.Record for the given topic.
// Extra publish headers are merged on top of the message headers.
func buildRecord(topic string, msg *broker.Message, extra broker.Headers) *kgo.Record {
	record := &kgo.Record{Topic: topic}

	if msg != nil {
		if msg.Key != "" {
			record.Key = []byte(msg.Key)
		}
		record.Value = msg.Body

		// Message-level headers.
		for k, v := range msg.Headers {
			record.Headers = append(record.Headers, kgo.RecordHeader{Key: k, Value: []byte(v)})
		}
	}

	// Publish-option headers override message headers.
	for k, v := range extra {
		record.Headers = append(record.Headers, kgo.RecordHeader{Key: k, Value: []byte(v)})
	}

	return record
}
