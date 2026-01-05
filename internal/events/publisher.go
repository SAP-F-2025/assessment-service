package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-kafka/v2/pkg/kafka"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/redis/go-redis/v9"
)

// EventPublisher defines the interface for publishing notification events
type EventPublisher interface {
	PublishNotificationEvent(ctx context.Context, event *NotificationEvent) error
	Close() error
}

// KafkaEventPublisher implements EventPublisher using Watermill with Kafka
type KafkaEventPublisher struct {
	publisher message.Publisher
	logger    *slog.Logger
	topicName string
}

// PublisherConfig holds configuration for the event publisher
type PublisherConfig struct {
	KafkaBrokers []string
	TopicName    string
	Logger       *slog.Logger
}

// NewKafkaEventPublisher creates a new Kafka-based event publisher using Watermill
func NewKafkaEventPublisher(config PublisherConfig) (*KafkaEventPublisher, error) {
	logger := watermill.NewSlogLogger(config.Logger)

	// Create Kafka publisher configuration
	publisherConfig := kafka.PublisherConfig{
		Brokers:   config.KafkaBrokers,
		Marshaler: kafka.DefaultMarshaler{},
	}

	// Create the publisher
	publisher, err := kafka.NewPublisher(publisherConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka publisher: %w", err)
	}

	return &KafkaEventPublisher{
		publisher: publisher,
		logger:    config.Logger,
		topicName: config.TopicName,
	}, nil
}

// PublishNotificationEvent publishes a notification event to Kafka
func (p *KafkaEventPublisher) PublishNotificationEvent(ctx context.Context, event *NotificationEvent) error {
	// Marshal the event to JSON
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal notification event: %w", err)
	}

	// Create Watermill message
	msg := message.NewMessage(event.ID, eventBytes)

	// Add metadata headers
	msg.Metadata.Set("event_type", string(event.Type))
	msg.Metadata.Set("source", event.Source)
	msg.Metadata.Set("version", event.Version)
	msg.Metadata.Set("timestamp", event.Timestamp.Format("2006-01-02T15:04:05Z07:00"))

	// Publish the message
	if err := p.publisher.Publish(p.topicName, msg); err != nil {
		p.logger.Error("Failed to publish notification event",
			"event_id", event.ID,
			"event_type", event.Type,
			"error", err)
		return fmt.Errorf("failed to publish notification event: %w", err)
	}

	p.logger.Info("Published notification event",
		"event_id", event.ID,
		"event_type", event.Type,
		"topic", p.topicName)

	return nil
}

// Close closes the publisher and releases resources
func (p *KafkaEventPublisher) Close() error {
	return p.publisher.Close()
}

// MockEventPublisher is a mock implementation for testing
type MockEventPublisher struct {
	Events []NotificationEvent
	Logger *slog.Logger
}

// NewMockEventPublisher creates a new mock event publisher
func NewMockEventPublisher(logger *slog.Logger) *MockEventPublisher {
	return &MockEventPublisher{
		Events: make([]NotificationEvent, 0),
		Logger: logger,
	}
}

// PublishNotificationEvent stores the event in memory (for testing)
func (m *MockEventPublisher) PublishNotificationEvent(ctx context.Context, event *NotificationEvent) error {
	m.Events = append(m.Events, *event)
	m.Logger.Info("Mock: Published notification event",
		"event_id", event.ID,
		"event_type", event.Type)
	return nil
}

// Close is a no-op for the mock publisher
func (m *MockEventPublisher) Close() error {
	return nil
}

// GetPublishedEvents returns all published events (for testing)
func (m *MockEventPublisher) GetPublishedEvents() []NotificationEvent {
	return m.Events
}

// ClearEvents clears all published events (for testing)
func (m *MockEventPublisher) ClearEvents() {
	m.Events = make([]NotificationEvent, 0)
}

// ===== REDIS STREAM PUBLISHER =====

// RedisStreamEventPublisher implements EventPublisher using Redis Streams
type RedisStreamEventPublisher struct {
	client     *redis.Client
	logger     *slog.Logger
	streamName string
}

// RedisStreamConfig holds configuration for Redis Stream publisher
type RedisStreamConfig struct {
	Client     *redis.Client
	StreamName string
	Logger     *slog.Logger
}

// NewRedisStreamEventPublisher creates a new Redis Stream-based event publisher
func NewRedisStreamEventPublisher(config RedisStreamConfig) *RedisStreamEventPublisher {
	streamName := config.StreamName
	if streamName == "" {
		streamName = "notifications" // default stream name
	}

	return &RedisStreamEventPublisher{
		client:     config.Client,
		logger:     config.Logger,
		streamName: streamName,
	}
}

// PublishNotificationEvent publishes a notification event to Redis Stream using XADD
func (p *RedisStreamEventPublisher) PublishNotificationEvent(ctx context.Context, event *NotificationEvent) error {
	if p.client == nil {
		p.logger.Warn("Redis client is nil, skipping event publish",
			"event_id", event.ID,
			"event_type", event.Type)
		return nil
	}

	// Marshal the event data to JSON
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal notification event: %w", err)
	}

	// Prepare fields for XADD
	// Using a map with structured fields for easier consumption
	fields := map[string]interface{}{
		"id":        event.ID,
		"type":      string(event.Type),
		"source":    event.Source,
		"version":   event.Version,
		"timestamp": event.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		"data":      string(eventBytes), // Full event JSON for consumers
	}

	// Add metadata if present
	if event.Metadata != nil {
		metadataBytes, err := json.Marshal(event.Metadata)
		if err == nil {
			fields["metadata"] = string(metadataBytes)
		}
	}

	// Use XADD to add the event to the stream
	// "*" means Redis will auto-generate the message ID
	result, err := p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: p.streamName,
		ID:     "*",
		Values: fields,
	}).Result()

	if err != nil {
		p.logger.Error("Failed to publish notification event to Redis Stream",
			"event_id", event.ID,
			"event_type", event.Type,
			"stream", p.streamName,
			"error", err)
		return fmt.Errorf("failed to publish notification event to Redis Stream: %w", err)
	}

	p.logger.Info("Published notification event to Redis Stream",
		"event_id", event.ID,
		"event_type", event.Type,
		"stream", p.streamName,
		"message_id", result)

	return nil
}

// Close is a no-op for Redis Stream publisher (connection managed externally)
func (p *RedisStreamEventPublisher) Close() error {
	// Redis client lifecycle is managed by the application, not by this publisher
	return nil
}

// GetStreamName returns the configured stream name
func (p *RedisStreamEventPublisher) GetStreamName() string {
	return p.streamName
}
