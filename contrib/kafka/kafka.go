// Package kafka wires Servora Kafka config to franz-go native clients.
package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	kafkapb "github.com/Servora-Kit/servora/api/gen/go/servora/contrib/kafka/v1"
	svrtls "github.com/Servora-Kit/servora/security/tls"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"github.com/twmb/franz-go/plugin/kotel"
)

// BuildOpts maps Kafka proto config into franz-go options and appends caller
// supplied options. Callers can reuse the mapping while adding consumer or
// producer specific options such as kgo.ConsumerGroup or kgo.ConsumeTopics.
func BuildOpts(cfg *kafkapb.Kafka, l *slog.Logger, extra ...kgo.Opt) ([]kgo.Opt, error) {
	if cfg == nil {
		return nil, fmt.Errorf("kafka: config must not be nil")
	}
	if len(cfg.GetBrokers()) == 0 {
		return nil, fmt.Errorf("kafka: at least one broker address is required")
	}
	if err := cfg.ApplyConf(); err != nil {
		return nil, err
	}

	opts := []kgo.Opt{kgo.SeedBrokers(cfg.GetBrokers()...)}
	if cfg.GetClientId() != "" {
		opts = append(opts, kgo.ClientID(cfg.GetClientId()))
	}
	if l != nil {
		opts = append(opts, kgo.WithLogger(slogAdapter{log: l.With("scope", "kafka/contrib")}))
	}

	if d := cfg.GetDialTimeout(); d != nil && d.AsDuration() > 0 {
		opts = append(opts, kgo.DialTimeout(d.AsDuration()))
	}
	if d := cfg.GetReadTimeout(); d != nil && d.AsDuration() > 0 {
		opts = append(opts, kgo.RetryTimeout(d.AsDuration()))
	}
	if d := cfg.GetWriteTimeout(); d != nil && d.AsDuration() > 0 {
		opts = append(opts, kgo.ProduceRequestTimeout(d.AsDuration()))
	}
	if d := cfg.GetRetryBackoff(); d != nil && d.AsDuration() > 0 {
		backoff := d.AsDuration()
		opts = append(opts, kgo.RetryBackoffFn(func(int) time.Duration { return backoff }))
	}
	if cfg.GetRetryMax() > 0 {
		opts = append(opts, kgo.RequestRetries(int(cfg.GetRetryMax())), kgo.RecordRetries(int(cfg.GetRetryMax())))
	}
	if cfg.GetRequiredAcks() != 0 {
		acks, err := requiredAcks(cfg.GetRequiredAcks())
		if err != nil {
			return nil, err
		}
		opts = append(opts, kgo.RequiredAcks(acks))
	}
	if cfg.GetCompression() != "" {
		compression, err := compressionCodec(cfg.GetCompression())
		if err != nil {
			return nil, err
		}
		opts = append(opts, kgo.ProducerBatchCompression(compression))
	}
	if cfg.GetTls().GetEnable() {
		tlsCfg, err := svrtls.NewClientConfig(svrtls.ClientConfigOptions{
			CAPath:   cfg.GetTls().GetCaPath(),
			CertPath: cfg.GetTls().GetCertPath(),
			KeyPath:  cfg.GetTls().GetKeyPath(),
		})
		if err != nil {
			return nil, fmt.Errorf("kafka: build TLS config: %w", err)
		}
		opts = append(opts, kgo.DialTLSConfig(tlsCfg))
	}
	if sasl := cfg.GetSasl(); sasl != nil {
		saslOpt, err := buildSASL(sasl)
		if err != nil {
			return nil, err
		}
		opts = append(opts, saslOpt)
	}

	kotelSvc := kotel.NewKotel(kotel.WithTracer(kotel.NewTracer()), kotel.WithMeter(kotel.NewMeter()))
	opts = append(opts, kgo.WithHooks(kotelSvc.Hooks()...))
	opts = append(opts, extra...)
	return opts, nil
}

// NewClient constructs a franz-go client from Kafka config and validates it
// with Ping. The caller owns the returned client's Close lifecycle.
func NewClient(ctx context.Context, cfg *kafkapb.Kafka, l *slog.Logger, extra ...kgo.Opt) (*kgo.Client, error) {
	opts, err := BuildOpts(cfg, l, extra...)
	if err != nil {
		return nil, err
	}
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("kafka: create client: %w", err)
	}
	if err := client.Ping(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("kafka: ping: %w", err)
	}
	return client, nil
}

// NewClientOptional returns nil when Kafka is absent. A configured Kafka client
// must connect successfully, otherwise the error is returned to the owner.
func NewClientOptional(ctx context.Context, cfg *kafkapb.Kafka, l *slog.Logger, extra ...kgo.Opt) (*kgo.Client, error) {
	log := loggerOrDefault(l).With("scope", "kafka/contrib")
	if cfg == nil || len(cfg.GetBrokers()) == 0 {
		log.Info("Kafka not configured")
		return nil, nil
	}
	client, err := NewClient(ctx, cfg, l, extra...)
	if err != nil {
		return nil, err
	}
	log.Info("Kafka connected")
	return client, nil
}

func buildSASL(sasl *kafkapb.Kafka_SASL) (kgo.Opt, error) {
	switch strings.ToUpper(strings.TrimSpace(sasl.GetMechanism())) {
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

func requiredAcks(v int32) (kgo.Acks, error) {
	switch v {
	case -1:
		return kgo.AllISRAcks(), nil
	case 0:
		return kgo.NoAck(), nil
	case 1:
		return kgo.LeaderAck(), nil
	default:
		return kgo.Acks{}, fmt.Errorf("kafka: unsupported required_acks %d", v)
	}
}

func compressionCodec(raw string) (kgo.CompressionCodec, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "none":
		return kgo.NoCompression(), nil
	case "snappy":
		return kgo.SnappyCompression(), nil
	case "gzip":
		return kgo.GzipCompression(), nil
	case "lz4":
		return kgo.Lz4Compression(), nil
	case "zstd":
		return kgo.ZstdCompression(), nil
	default:
		return kgo.NoCompression(), fmt.Errorf("kafka: unsupported compression %q", raw)
	}
}

func loggerOrDefault(l *slog.Logger) *slog.Logger {
	if l != nil {
		return l
	}
	return slog.Default()
}

type slogAdapter struct {
	log *slog.Logger
}

func (a slogAdapter) Level() kgo.LogLevel {
	if !a.log.Enabled(context.Background(), slog.LevelDebug) {
		return kgo.LogLevelInfo
	}
	return kgo.LogLevelDebug
}

func (a slogAdapter) Log(level kgo.LogLevel, msg string, keyvals ...any) {
	switch level {
	case kgo.LogLevelDebug:
		a.log.Debug(msg, keyvals...)
	case kgo.LogLevelInfo:
		a.log.Info(msg, keyvals...)
	case kgo.LogLevelWarn:
		a.log.Warn(msg, keyvals...)
	case kgo.LogLevelError:
		a.log.Error(msg, keyvals...)
	}
}
