package services

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/events"
	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/validator"
)

// MockRepository for testing
type MockRepository struct{}

func (m *MockRepository) Assessment() interface{} { return nil }
func (m *MockRepository) Attempt() interface{}    { return nil }

func TestNotificationEventService_PublishEvents(t *testing.T) {
	// Setup
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockPublisher := events.NewMockEventPublisher(logger)
	validator := &validator.Validator{} // Assuming this exists
	mockRepo := &MockRepository{}

	// Create service
	service := NewNotificationEventService(
		mockRepo,
		mockPublisher,
		logger,
		validator,
	)

	ctx := context.Background()

	t.Run("SendBulkNotification", func(t *testing.T) {
		// Test bulk notification
		userIDs := []uint{1, 2, 3}
		notification := &NotificationRequest{
			Type:     models.NotificationAssessmentPublished,
			Title:    "Test Notification",
			Message:  "This is a test message",
			Priority: models.PriorityHigh,
		}

		err := service.SendBulkNotification(ctx, userIDs, notification)
		if err != nil {
			t.Fatalf("Failed to send bulk notification: %v", err)
		}

		// Verify event was published
		events := mockPublisher.GetPublishedEvents()
		if len(events) != 1 {
			t.Fatalf("Expected 1 event, got %d", len(events))
		}

		event := events[0]
		if event.Type != events.EventBulkNotification {
			t.Errorf("Expected event type %s, got %s", events.EventBulkNotification, event.Type)
		}

		// Check event data
		if eventData, ok := event.Data.(events.BulkNotificationEvent); ok {
			if len(eventData.RecipientIDs) != 3 {
				t.Errorf("Expected 3 recipients, got %d", len(eventData.RecipientIDs))
			}
			if eventData.Title != "Test Notification" {
				t.Errorf("Expected title 'Test Notification', got '%s'", eventData.Title)
			}
		} else {
			t.Error("Event data is not BulkNotificationEvent type")
		}
	})

	t.Run("Event_Structure_Validation", func(t *testing.T) {
		mockPublisher.ClearEvents()

		// Test event structure for bulk notification
		userIDs := []uint{123}
		notification := &NotificationRequest{
			Type:     models.NotificationAssessmentDue,
			Title:    "Assessment Due Soon",
			Message:  "Your assessment is due in 2 hours",
			Priority: models.PriorityNormal,
		}

		err := service.SendBulkNotification(ctx, userIDs, notification)
		if err != nil {
			t.Fatalf("Failed to send notification: %v", err)
		}

		events := mockPublisher.GetPublishedEvents()
		if len(events) != 1 {
			t.Fatalf("Expected 1 event, got %d", len(events))
		}

		event := events[0]

		// Validate event structure
		if event.ID == "" {
			t.Error("Event ID should not be empty")
		}
		if event.Source != "assessment-service" {
			t.Errorf("Expected source 'assessment-service', got '%s'", event.Source)
		}
		if event.Version != "1.0" {
			t.Errorf("Expected version '1.0', got '%s'", event.Version)
		}
		if event.Timestamp.IsZero() {
			t.Error("Event timestamp should not be zero")
		}
	})
}

// Integration test example (would require actual Kafka)
func TestNotificationEventService_KafkaIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// This test would require a running Kafka instance
	// You could use testcontainers-go to spin up Kafka for integration testing

	t.Log("Integration test would:")
	t.Log("1. Start Kafka container")
	t.Log("2. Create KafkaEventPublisher")
	t.Log("3. Publish events")
	t.Log("4. Verify events are received by consumer")
	t.Log("5. Cleanup Kafka container")
}

// Benchmark test
func BenchmarkNotificationEventService_PublishEvent(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockPublisher := events.NewMockEventPublisher(logger)
	validator := &validator.Validator{}
	mockRepo := &MockRepository{}

	service := NewNotificationEventService(
		mockRepo,
		mockPublisher,
		logger,
		validator,
	)

	ctx := context.Background()
	userIDs := []uint{1, 2, 3}
	notification := &NotificationRequest{
		Type:     models.NotificationAssessmentPublished,
		Title:    "Benchmark Test",
		Message:  "Benchmark message",
		Priority: models.PriorityNormal,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := service.SendBulkNotification(ctx, userIDs, notification)
		if err != nil {
			b.Fatalf("Failed to send notification: %v", err)
		}
	}
}
