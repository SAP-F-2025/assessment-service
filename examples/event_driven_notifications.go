package examples

import (
	"context"
	"log/slog"
	"os"

	"github.com/SAP-F-2025/assessment-service/internal/config"
	"github.com/SAP-F-2025/assessment-service/internal/events"
	"github.com/SAP-F-2025/assessment-service/internal/services"
	"gorm.io/gorm"
)

// ExampleEventDrivenNotificationSetup shows how to set up the event-driven notification system
func ExampleEventDrivenNotificationSetup(db *gorm.DB, repo interface{}, logger *slog.Logger) error {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	// Create event publisher
	eventPublisher, err := cfg.Events.CreateEventPublisher(logger)
	if err != nil {
		logger.Error("Failed to create event publisher", "error", err)
		// Fallback to mock publisher for development
		eventPublisher = events.NewMockEventPublisher(logger)
	}

	// Create notification event service
	// Note: You'll need to cast repo to your actual repository interface
	// notificationService := services.NewNotificationEventService(
	// 	repo.(repositories.Repository),
	// 	eventPublisher,
	// 	logger,
	// 	validator,
	// )

	// Example usage:
	ctx := context.Background()

	// Publish an assessment published event
	// err = notificationService.NotifyAssessmentPublished(ctx, 123)
	// if err != nil {
	// 	logger.Error("Failed to publish assessment published event", "error", err)
	// }

	logger.Info("Event-driven notification system initialized successfully")

	// Don't forget to close the publisher on shutdown
	defer func() {
		if err := eventPublisher.Close(); err != nil {
			logger.Error("Failed to close event publisher", "error", err)
		}
	}()

	return nil
}

// ExampleKafkaSetup shows minimal Kafka setup for development/testing
func ExampleKafkaSetup() {
	// Set environment variables for Kafka setup
	os.Setenv("EVENTS_ENABLED", "true")
	os.Setenv("EVENTS_PUBLISHER", "kafka")
	os.Setenv("KAFKA_BROKERS", "localhost:9092")
	os.Setenv("NOTIFICATION_TOPIC", "assessment_notifications")
}

// ExampleMockSetup shows how to use mock publisher for testing
func ExampleMockSetup() {
	// Set environment variables for mock setup
	os.Setenv("EVENTS_ENABLED", "true")
	os.Setenv("EVENTS_PUBLISHER", "mock")
}

// ExampleEventHandling shows how events would be consumed by external notification service
// This would typically be in a separate service/application
func ExampleEventHandling() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// This is what your external notification service would do:
	// 1. Set up Kafka consumer using Watermill
	// 2. Subscribe to the notification topic
	// 3. Handle incoming events and send actual notifications (email, push, SMS, etc.)

	logger.Info("Example: External notification service would:")
	logger.Info("1. Listen to Kafka topic: assessment_notifications")
	logger.Info("2. Process events like:",
		"event_types", []string{
			"assessment.published",
			"attempt.started",
			"attempt.submitted",
			"grading.completed",
		})
	logger.Info("3. Send notifications via:")
	logger.Info("   - Email service")
	logger.Info("   - Push notification service")
	logger.Info("   - SMS service")
	logger.Info("   - In-app notification system")
}

// ExampleNotificationServiceUsage shows how to use the service in your handlers
func ExampleNotificationServiceUsage() {
	// In your HTTP handlers or service methods, replace direct notification calls with event publishing:

	// OLD WAY (direct notification service):
	// notificationService.NotifyAssessmentPublished(ctx, assessmentID)

	// NEW WAY (event-driven):
	// notificationEventService.NotifyAssessmentPublished(ctx, assessmentID)
	// This publishes an event to Kafka, which is then consumed by external notification service

	// Benefits:
	// - Decoupled: Assessment service doesn't need to know about notification implementations
	// - Resilient: If notification service is down, events are queued in Kafka
	// - Scalable: Multiple notification service instances can consume events
	// - Flexible: Easy to add new notification channels without changing assessment service
	// - Auditable: All notification events are logged in Kafka
}
