package services

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	// DefaultWorkerInterval is the default interval for the status worker
	DefaultWorkerInterval = 30 * time.Second
	// DefaultGracePeriod is the default grace period before marking attempt as timeout
	DefaultGracePeriod = 60 * time.Second
	// WorkerLockKey is the Redis key for distributed lock
	WorkerLockKey = "status_worker:lock"
	// WorkerLockTTL is the TTL for the distributed lock
	WorkerLockTTL = 25 * time.Second
)

// StatusWorkerConfig holds configuration for the status worker
type StatusWorkerConfig struct {
	Enabled                   bool
	Interval                  time.Duration
	GracePeriod               time.Duration
	ExpiringNotificationHours []int // Hours before due_date to send reminders (e.g., [24, 1])
}

// DefaultStatusWorkerConfig returns the default configuration
func DefaultStatusWorkerConfig() StatusWorkerConfig {
	return StatusWorkerConfig{
		Enabled:                   true,
		Interval:                  DefaultWorkerInterval,
		GracePeriod:               DefaultGracePeriod,
		ExpiringNotificationHours: []int{24, 1}, // Default: 24h and 1h before due_date
	}
}

// StatusWorker handles automatic status updates for assessments and attempts
type StatusWorker struct {
	db                  *gorm.DB
	repo                repositories.Repository
	logger              *slog.Logger
	redisClient         *redis.Client
	attemptService      AttemptService
	notificationService NotificationEventService
	config              StatusWorkerConfig

	stopCh  chan struct{}
	wg      sync.WaitGroup
	running bool
	mu      sync.Mutex
}

// NewStatusWorker creates a new status worker
func NewStatusWorker(
	db *gorm.DB,
	repo repositories.Repository,
	logger *slog.Logger,
	redisClient *redis.Client,
	attemptService AttemptService,
	config StatusWorkerConfig,
) *StatusWorker {
	return &StatusWorker{
		db:             db,
		repo:           repo,
		logger:         logger,
		redisClient:    redisClient,
		attemptService: attemptService,
		config:         config,
		stopCh:         make(chan struct{}),
	}
}

// NewStatusWorkerWithNotification creates a status worker with notification support
func NewStatusWorkerWithNotification(
	db *gorm.DB,
	repo repositories.Repository,
	logger *slog.Logger,
	redisClient *redis.Client,
	attemptService AttemptService,
	notificationService NotificationEventService,
	config StatusWorkerConfig,
) *StatusWorker {
	return &StatusWorker{
		db:                  db,
		repo:                repo,
		logger:              logger,
		redisClient:         redisClient,
		attemptService:      attemptService,
		notificationService: notificationService,
		config:              config,
		stopCh:              make(chan struct{}),
	}
}

// Start begins the background worker
func (w *StatusWorker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("status worker is already running")
	}
	w.running = true
	w.mu.Unlock()

	w.logger.Info("Starting status worker",
		"interval", w.config.Interval,
		"grace_period", w.config.GracePeriod,
	)

	w.wg.Add(1)
	go w.run(ctx)

	return nil
}

// Stop gracefully stops the background worker
func (w *StatusWorker) Stop() error {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return nil
	}
	w.mu.Unlock()

	w.logger.Info("Stopping status worker...")
	close(w.stopCh)
	w.wg.Wait()

	w.mu.Lock()
	w.running = false
	w.stopCh = make(chan struct{}) // Reset for potential restart
	w.mu.Unlock()

	w.logger.Info("Status worker stopped")
	return nil
}

// run is the main worker loop
func (w *StatusWorker) run(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()

	// Run immediately on start
	w.processTick(ctx)

	for {
		select {
		case <-ticker.C:
			w.processTick(ctx)
		case <-w.stopCh:
			w.logger.Info("Status worker received stop signal")
			return
		case <-ctx.Done():
			w.logger.Info("Status worker context cancelled")
			return
		}
	}
}

// processTick handles one iteration of the worker
func (w *StatusWorker) processTick(ctx context.Context) {
	// Try to acquire distributed lock if Redis is available
	if w.redisClient != nil {
		acquired, err := w.acquireLock(ctx)
		if err != nil {
			w.logger.Warn("Failed to acquire lock", "error", err)
			// Continue anyway - idempotent operations
		}
		if !acquired {
			w.logger.Debug("Another instance is processing, skipping")
			return
		}
		defer w.releaseLock(ctx)
	}

	w.logger.Debug("Processing status worker tick")

	// Process timed out attempts (with grace period)
	if err := w.processTimedOutAttempts(ctx); err != nil {
		w.logger.Error("Failed to process timed out attempts", "error", err)
	}

	// Process expired assessments
	if err := w.processExpiredAssessments(ctx); err != nil {
		w.logger.Error("Failed to process expired assessments", "error", err)
	}

	// Process expiring assessment notifications
	if w.notificationService != nil && len(w.config.ExpiringNotificationHours) > 0 {
		if err := w.processExpiringAssessments(ctx); err != nil {
			w.logger.Error("Failed to process expiring assessment notifications", "error", err)
		}
	}
}

// processTimedOutAttempts handles attempts that have exceeded their time limit + grace period
func (w *StatusWorker) processTimedOutAttempts(ctx context.Context) error {
	// Calculate cutoff time: now - grace period
	// If ended_at < cutoffTime, the attempt has exceeded the grace period
	cutoffTime := time.Now().Add(-w.config.GracePeriod)

	// Find attempts where:
	// - status = 'in_progress'
	// - ended_at IS NOT NULL AND ended_at < cutoffTime (exceeded grace period)
	var attempts []*models.AssessmentAttempt
	err := w.db.WithContext(ctx).
		Where("status = ? AND ended_at IS NOT NULL AND ended_at < ?", models.AttemptInProgress, cutoffTime).
		Find(&attempts).Error

	if err != nil {
		return fmt.Errorf("failed to get timed out attempts: %w", err)
	}

	if len(attempts) == 0 {
		return nil
	}

	w.logger.Info("Processing timed out attempts",
		"count", len(attempts),
		"cutoff_time", cutoffTime,
	)

	for _, attempt := range attempts {
		if err := w.attemptService.HandleTimeout(ctx, attempt.ID); err != nil {
			w.logger.Error("Failed to timeout attempt",
				"attempt_id", attempt.ID,
				"student_id", attempt.StudentID,
				"error", err,
			)
			continue
		}

		w.logger.Info("Attempt timed out and auto-graded",
			"attempt_id", attempt.ID,
			"student_id", attempt.StudentID,
			"assessment_id", attempt.AssessmentID,
		)
	}

	return nil
}

// processExpiredAssessments handles assessments that have passed their due date + grace period
func (w *StatusWorker) processExpiredAssessments(ctx context.Context) error {
	// Calculate cutoff time (now - grace period)
	cutoffTime := time.Now().Add(-w.config.GracePeriod)

	// Find assessments where:
	// - status = 'Active'
	// - due_date IS NOT NULL AND due_date < cutoffTime
	var assessments []*models.Assessment
	err := w.db.WithContext(ctx).
		Where("status = ? AND due_date IS NOT NULL AND due_date < ?", models.StatusActive, cutoffTime).
		Find(&assessments).Error

	if err != nil {
		return fmt.Errorf("failed to get expired assessments: %w", err)
	}

	if len(assessments) == 0 {
		return nil
	}

	w.logger.Info("Processing expired assessments",
		"count", len(assessments),
		"cutoff_time", cutoffTime,
	)

	for _, assessment := range assessments {
		if err := w.expireAssessment(ctx, assessment); err != nil {
			w.logger.Error("Failed to expire assessment",
				"assessment_id", assessment.ID,
				"error", err,
			)
			continue
		}

		w.logger.Info("Assessment expired",
			"assessment_id", assessment.ID,
			"title", assessment.Title,
			"due_date", assessment.DueDate,
		)
	}

	return nil
}

// expireAssessment marks an assessment as expired and timeouts all in-progress attempts
func (w *StatusWorker) expireAssessment(ctx context.Context, assessment *models.Assessment) error {
	// First, update assessment status to Expired
	if err := w.repo.Assessment().UpdateStatus(ctx, nil, assessment.ID, models.StatusExpired); err != nil {
		return fmt.Errorf("failed to update assessment status: %w", err)
	}

	// Find all in-progress attempts for this assessment
	var inProgressAttempts []*models.AssessmentAttempt
	if err := w.db.WithContext(ctx).
		Where("assessment_id = ? AND status = ?", assessment.ID, models.AttemptInProgress).
		Find(&inProgressAttempts).Error; err != nil {
		return fmt.Errorf("failed to get in-progress attempts: %w", err)
	}

	// Timeout each attempt using HandleTimeout (includes auto-grading)
	for _, attempt := range inProgressAttempts {
		if err := w.attemptService.HandleTimeout(ctx, attempt.ID); err != nil {
			w.logger.Error("Failed to timeout attempt during assessment expiry",
				"attempt_id", attempt.ID,
				"assessment_id", assessment.ID,
				"error", err,
			)
			continue
		}

		w.logger.Info("Attempt timed out due to assessment expiry",
			"attempt_id", attempt.ID,
			"assessment_id", assessment.ID,
		)
	}

	return nil
}

// acquireLock attempts to acquire a distributed lock using Redis
func (w *StatusWorker) acquireLock(ctx context.Context) (bool, error) {
	if w.redisClient == nil {
		return true, nil // No Redis, assume lock acquired
	}

	result, err := w.redisClient.SetNX(ctx, WorkerLockKey, "locked", WorkerLockTTL).Result()
	if err != nil {
		return false, fmt.Errorf("failed to set lock: %w", err)
	}

	return result, nil
}

// releaseLock releases the distributed lock
func (w *StatusWorker) releaseLock(ctx context.Context) {
	if w.redisClient == nil {
		return
	}

	if err := w.redisClient.Del(ctx, WorkerLockKey).Err(); err != nil {
		w.logger.Warn("Failed to release lock", "error", err)
	}
}

// IsRunning returns whether the worker is currently running
func (w *StatusWorker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// processExpiringAssessments sends reminder notifications for assessments approaching their due_date
// Uses Redis keys for idempotency to prevent duplicate notifications across multiple instances
func (w *StatusWorker) processExpiringAssessments(ctx context.Context) error {
	if w.notificationService == nil {
		return nil
	}

	now := time.Now()

	for _, hours := range w.config.ExpiringNotificationHours {
		// Calculate time window: assessments with due_date between now and now + hours
		// We use a small window (worker interval) to avoid re-checking same assessments
		windowStart := now.Add(time.Duration(hours)*time.Hour - w.config.Interval)
		windowEnd := now.Add(time.Duration(hours) * time.Hour)

		// Find active assessments with due_date in this window
		var assessments []*models.Assessment
		err := w.db.WithContext(ctx).
			Where("status = ? AND due_date IS NOT NULL AND due_date > ? AND due_date <= ?",
				models.StatusActive, windowStart, windowEnd).
			Find(&assessments).Error

		if err != nil {
			w.logger.Error("Failed to query expiring assessments", "hours", hours, "error", err)
			continue
		}

		for _, assessment := range assessments {
			// Get groups assigned to this assessment
			groups, err := w.repo.AssessmentGroup().GetGroupsByAssessment(ctx, nil, assessment.ID)
			if err != nil {
				w.logger.Error("Failed to get groups for assessment",
					"assessment_id", assessment.ID, "error", err)
				continue
			}

			for _, group := range groups {
				// Check idempotency key in Redis to prevent duplicate notifications
				idempotencyKey := fmt.Sprintf("expiring_notification:%d:%d:%dh", assessment.ID, group.ID, hours)

				if w.redisClient != nil {
					// Use SetNX with 25h TTL (longer than max reminder interval)
					wasSet, err := w.redisClient.SetNX(ctx, idempotencyKey, "sent", 25*time.Hour).Result()
					if err != nil {
						w.logger.Warn("Failed to check idempotency key", "key", idempotencyKey, "error", err)
						// Continue anyway, may result in duplicate but better than missing
					} else if !wasSet {
						// Already sent this notification
						w.logger.Debug("Skipping duplicate expiring notification",
							"assessment_id", assessment.ID, "group_id", group.ID, "hours", hours)
						continue
					}
				}

				// Send notification
				if err := w.notificationService.NotifyAssessmentExpiring(ctx, assessment.ID, group.ID, hours); err != nil {
					w.logger.Error("Failed to send expiring notification",
						"assessment_id", assessment.ID,
						"group_id", group.ID,
						"hours", hours,
						"error", err)
					// Delete the idempotency key so we can retry
					if w.redisClient != nil {
						w.redisClient.Del(ctx, idempotencyKey)
					}
					continue
				}

				w.logger.Info("Sent assessment expiring notification",
					"assessment_id", assessment.ID,
					"assessment_title", assessment.Title,
					"group_id", group.ID,
					"hours_remaining", hours,
					"due_date", assessment.DueDate)
			}
		}
	}

	return nil
}
