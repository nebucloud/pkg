package logger

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	slogkafka "github.com/samber/slog-kafka"
	"github.com/segmentio/kafka-go"
	"k8s.io/klog"
)

// KlogWrapper wraps klog for use with slog.
type KlogWrapper struct {
	attrs []slog.Attr
	group string
}

// WithAttrs implements slog.Handler.
func (kw *KlogWrapper) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(kw.attrs)+len(attrs))
	copy(newAttrs, kw.attrs)
	copy(newAttrs[len(kw.attrs):], attrs)
	return &KlogWrapper{
		attrs: newAttrs,
		group: kw.group,
	}
}

// WithGroup implements slog.Handler.
func (kw *KlogWrapper) WithGroup(name string) slog.Handler {
	return &KlogWrapper{
		attrs: kw.attrs,
		group: kw.group + "." + name,
	}
}

func (kw *KlogWrapper) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (kw *KlogWrapper) Handle(_ context.Context, r slog.Record) error {
	// Forward messages to klog
	attrs := make([]interface{}, 0)
	r.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr.Key, attr.Value)
		return true
	})
	msg := r.Message
	if kw.group != "" {
		attrs = append(attrs, "group", kw.group)
	}

	// Format attributes as JSON
	jsonAttrs := make(map[string]interface{})
	for i := 0; i < len(attrs); i += 2 {
		jsonAttrs[attrs[i].(string)] = attrs[i+1]
	}
	jsonData, _ := json.Marshal(jsonAttrs)

	klog.Infof("%s %s", msg, string(jsonData))
	return nil
}

// NewKlogHandler creates a new slog.Handler that forwards to klog.
func NewKlogHandler() slog.Handler {
	return &KlogWrapper{}
}

// / SetupKafkaWriter initializes and returns a Kafka writer.
func SetupKafkaWriter(brokers []string) *kafka.Writer {
	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}

	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:     brokers,
		Topic:       "logs",
		Dialer:      dialer,
		Async:       true, // Async mode
		Balancer:    &kafka.Hash{},
		MaxAttempts: 3,
	})

	return writer
}

// NewKafkaHandler creates a new slog.Handler that forwards to Kafka.
func NewKafkaHandler(brokers []string) slog.Handler {
	kafkaWriter := SetupKafkaWriter(brokers)
	return slogkafka.Option{
		Level:       slog.LevelDebug,
		KafkaWriter: kafkaWriter,
	}.NewKafkaHandler()
}
