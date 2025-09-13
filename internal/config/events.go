package config

import (
	"log/slog"
	"strings"

	"github.com/SAP-F-2025/assessment-service/internal/events"
)

// EventConfig holds configuration for event publishing
type EventConfig struct {
	Enabled           bool   `env:"EVENTS_ENABLED" envDefault:"true"`
	Publisher         string `env:"EVENTS_PUBLISHER" envDefault:"kafka"` // kafka or mock
	KafkaBrokers      string `env:"KAFKA_BROKERS" envDefault:"localhost:9092"`
	NotificationTopic string `env:"NOTIFICATION_TOPIC" envDefault:"notifications"`
}

// GetKafkaBrokers returns Kafka brokers as a slice
func (c *EventConfig) GetKafkaBrokers() []string {
	return strings.Split(c.KafkaBrokers, ",")
}

// CreateEventPublisher creates an event publisher based on configuration
func (c *EventConfig) CreateEventPublisher(logger *slog.Logger) (events.EventPublisher, error) {
	if !c.Enabled {
		logger.Info("Event publishing disabled, using mock publisher")
		return events.NewMockEventPublisher(logger), nil
	}

	switch c.Publisher {
	case "kafka":
		logger.Info("Creating Kafka event publisher",
			"brokers", c.KafkaBrokers,
			"topic", c.NotificationTopic)

		return events.NewKafkaEventPublisher(events.PublisherConfig{
			KafkaBrokers: c.GetKafkaBrokers(),
			TopicName:    c.NotificationTopic,
			Logger:       logger,
		})
	case "mock":
		logger.Info("Using mock event publisher")
		return events.NewMockEventPublisher(logger), nil
	default:
		logger.Warn("Unknown event publisher type, falling back to mock", "publisher", c.Publisher)
		return events.NewMockEventPublisher(logger), nil
	}
}
