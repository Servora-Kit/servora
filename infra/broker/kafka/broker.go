// Package kafka provides a franz-go based implementation of pkg/broker.
// Reference: /Users/horonlee/projects/go/Kemate pkg/kafka patterns.
package kafka

import (
	"context"
	"fmt"
	"strings"
	"sync"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/infra/broker"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"github.com/twmb/franz-go/plugin/kotel"
	"github.com/twmb/franz-go/plugin/kzap"
	"go.uber.org/zap"
)

var _ broker.Broker = (*kafkaBroker)(nil)

// kafkaBroker implements broker.Broker backed by franz-go.
type kafkaBroker struct {
	cfg    *conf.Data_Kafka
	zap    *zap.Logger
	client *kgo.Client

	mu          sync.RWMutex
	subscribers []*kafkaSubscriber
}

// NewBroker creates a kafkaBroker from proto config. Connect() must be called before use.
func NewBroker(cfg *conf.Data_Kafka, zapLogger *zap.Logger) (*kafkaBroker, error) {
	if cfg == nil {
		return nil, fmt.Errorf("kafka: config must not be nil")
	}
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka: at least one broker address is required")
	}
	return &kafkaBroker{cfg: cfg, zap: zapLogger}, nil
}

// Connect builds and validates the kgo.Client.
func (b *kafkaBroker) Connect(ctx context.Context) error {
	opts, err := b.buildOpts()
	if err != nil {
		return fmt.Errorf("kafka: build client opts: %w", err)
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("kafka: create client: %w", err)
	}

	// Ping the cluster to validate connectivity.
	if err := client.Ping(ctx); err != nil {
		client.Close()
		return fmt.Errorf("kafka: ping failed: %w", err)
	}

	b.mu.Lock()
	b.client = client
	b.mu.Unlock()
	return nil
}

// Disconnect closes the kgo.Client and all active subscribers.
func (b *kafkaBroker) Disconnect(_ context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, sub := range b.subscribers {
		_ = sub.Unsubscribe(false) // false: broker manages the slice, no re-locking
	}
	b.subscribers = nil

	if b.client != nil {
		b.client.Close()
		b.client = nil
	}
	return nil
}

// removeSub removes a subscriber from the internal list (called by Unsubscribe with removeFromManager=true).
func (b *kafkaBroker) removeSub(target *kafkaSubscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs := b.subscribers[:0]
	for _, s := range b.subscribers {
		if s != target {
			subs = append(subs, s)
		}
	}
	b.subscribers = subs
}

// Publish sends a message to the given topic (synchronous, waits for ack).
func (b *kafkaBroker) Publish(ctx context.Context, topic string, msg *broker.Message, opts ...broker.PublishOption) error {
	b.mu.RLock()
	client := b.client
	b.mu.RUnlock()
	if client == nil {
		return fmt.Errorf("kafka: not connected")
	}

	o := &broker.PublishOptions{}
	for _, opt := range opts {
		opt(o)
	}

	record := buildRecord(topic, msg, o.Headers)
	results := client.ProduceSync(ctx, record)
	return results.FirstErr()
}

// Subscribe starts a consumer group poll loop in a goroutine.
func (b *kafkaBroker) Subscribe(ctx context.Context, topic string, handler broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	sopts := broker.NewSubscribeOptions(opts...)
	// Wrap handler with any declared middleware (outermost first).
	if len(sopts.Middlewares) > 0 {
		handler = broker.Chain(handler, sopts.Middlewares...)
	}

	// Build a dedicated consumer client per subscription.
	consumerOpts, err := b.buildOpts()
	if err != nil {
		return nil, fmt.Errorf("kafka: build consumer opts: %w", err)
	}

	group := sopts.Queue
	if group == "" {
		group = b.cfg.GetConsumerGroup()
	}
	consumerOpts = append(consumerOpts,
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
	)

	consumerClient, err := kgo.NewClient(consumerOpts...)
	if err != nil {
		return nil, fmt.Errorf("kafka: create consumer client: %w", err)
	}

	sub := &kafkaSubscriber{
		topic:   topic,
		client:  consumerClient,
		handler: handler,
		sopts:   sopts,
		done:    make(chan struct{}),
		zap:     b.zap,
		broker:  b,
	}

	b.mu.Lock()
	b.subscribers = append(b.subscribers, sub)
	b.mu.Unlock()

	go sub.poll(ctx)
	return sub, nil
}

// buildOpts assembles kgo.Opt slice from proto config.
func (b *kafkaBroker) buildOpts() ([]kgo.Opt, error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(b.cfg.Brokers...),
	}

	if b.cfg.GetClientId() != "" {
		opts = append(opts, kgo.ClientID(b.cfg.GetClientId()))
	}

	// Logger bridge: route franz-go internal logs to Servora zap.
	if b.zap != nil {
		opts = append(opts, kgo.WithLogger(kzap.New(b.zap)))
	}

	// OTel hooks: auto-instrument produce/consume with tracing + metrics.
	ktracer := kotel.NewTracer()
	kmeter := kotel.NewMeter()
	kotelSvc := kotel.NewKotel(kotel.WithTracer(ktracer), kotel.WithMeter(kmeter))
	opts = append(opts, kgo.WithHooks(kotelSvc.Hooks()...))

	// SASL.
	if sasl := b.cfg.GetSasl(); sasl != nil {
		saslOpt, err := buildSASL(sasl)
		if err != nil {
			return nil, err
		}
		opts = append(opts, saslOpt)
	}

	return opts, nil
}

// buildSASL maps proto SASL config to a kgo SASL mechanism option.
func buildSASL(sasl *conf.Data_Kafka_SASL) (kgo.Opt, error) {
	switch strings.ToUpper(sasl.GetMechanism()) {
	case "PLAIN":
		return kgo.SASL(plain.Auth{
			User: sasl.GetUsername(),
			Pass: sasl.GetPassword(),
		}.AsMechanism()), nil
	case "SCRAM-SHA-256":
		return kgo.SASL(scram.Auth{
			User: sasl.GetUsername(),
			Pass: sasl.GetPassword(),
		}.AsSha256Mechanism()), nil
	case "SCRAM-SHA-512":
		return kgo.SASL(scram.Auth{
			User: sasl.GetUsername(),
			Pass: sasl.GetPassword(),
		}.AsSha512Mechanism()), nil
	default:
		return nil, fmt.Errorf("kafka: unsupported SASL mechanism %q", sasl.GetMechanism())
	}
}
