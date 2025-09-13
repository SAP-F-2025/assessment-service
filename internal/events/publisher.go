package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-kafka/v2/pkg/kafka"
	"github.com/ThreeDotsLabs/watermill/message"
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
