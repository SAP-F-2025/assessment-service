package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
)

// ValidationService provides centralized business rule validation
type ValidationService struct {
	repo repositories.Repository
}

func NewValidationService(repo repositories.Repository) *ValidationService {
	return &ValidationService{
		repo: repo,
	}
}

// ===== ASSESSMENT VALIDATION =====

func (v *ValidationService) ValidateAssessmentCreate(ctx context.Context, req *CreateAssessmentRequest, creatorID uint) ValidationErrors {
	var errors ValidationErrors

	// Validate basic fields (already handled by struct tags, but can add custom rules)
	errors = append(errors, v.validateAssessmentBasicFields(req.Title, req.Description, req.Duration, req.PassingScore, req.MaxAttempts)...)

	// Business rule validations
	errors = append(errors, v.validateAssessmentBusinessRules(ctx, req, creatorID, nil)...)

	// Validate settings if provided
	if req.Settings != nil {
		errors = append(errors, v.validateAssessmentSettings(req.Settings)...)
	}

	// Validate questions if provided
	if len(req.Questions) > 0 {
		errors = append(errors, v.validateAssessmentQuestions(ctx, req.Questions, creatorID)...)
	}

	return errors
}

func (v *ValidationService) ValidateAssessmentUpdate(ctx context.Context, req *UpdateAssessmentRequest, assessment *models.Assessment, userID uint) ValidationErrors {
	var errors ValidationErrors

	// Validate basic fields if being updated
	if req.Title != nil || req.Description != nil || req.Duration != nil || req.PassingScore != nil || req.MaxAttempts != nil {
		title := assessment.Title
		if req.Title != nil {
			title = *req.Title
		}

		description := assessment.Description
		if req.Description != nil {
			description = req.Description
		}

		duration := assessment.Duration
		if req.Duration != nil {
			duration = *req.Duration
		}

		passingScore := assessment.PassingScore
		if req.PassingScore != nil {
			passingScore = *req.PassingScore
		}

		maxAttempts := assessment.MaxAttempts
		if req.MaxAttempts != nil {
			maxAttempts = *req.MaxAttempts
		}

		errors = append(errors, v.validateAssessmentBasicFields(title, description, duration, passingScore, maxAttempts)...)
	}

	// Business rule validations for updates
	errors = append(errors, v.validateAssessmentUpdateRules(ctx, req, assessment, userID)...)

	// Validate settings if provided
	if req.Settings != nil {
		errors = append(errors, v.validateAssessmentSettings(req.Settings)...)
	}

	return errors
}

func (v *ValidationService) validateAssessmentBasicFields(title string, description *string, duration, passingScore, maxAttempts int) ValidationErrors {
	var errors ValidationErrors

	// Title validation
	if strings.TrimSpace(title) == "" {
		errors = append(errors, *NewValidationError("title", "cannot be empty", title))
	}

	if len(title) > 200 {
		errors = append(errors, *NewValidationError("title", "cannot exceed 200 characters", len(title)))
	}

	// Description validation
	if description != nil && len(*description) > 1000 {
		errors = append(errors, *NewValidationError("description", "cannot exceed 1000 characters", len(*description)))
	}

	// Duration validation
	if duration < 5 || duration > 300 {
		errors = append(errors, *NewValidationError("duration", "must be between 5 and 300 minutes", duration))
	}

	// Passing score validation
	if passingScore < 0 || passingScore > 100 {
		errors = append(errors, *NewValidationError("passing_score", "must be between 0 and 100", passingScore))
	}

	// Max attempts validation
	if maxAttempts < 1 || maxAttempts > 10 {
		errors = append(errors, *NewValidationError("max_attempts", "must be between 1 and 10", maxAttempts))
	}

	return errors
}

func (v *ValidationService) validateAssessmentBusinessRules(ctx context.Context, req *CreateAssessmentRequest, creatorID uint, existingAssessment *models.Assessment) ValidationErrors {
	var errors ValidationErrors

	// QT-006: Assessment title must be unique within creator's scope
	existing, err := v.repo.Assessment().GetByTitleAndCreator(ctx, req.Title, creatorID)
	if err != nil && !repositories.IsNotFoundError(err) {
		errors = append(errors, *NewValidationError("title", "failed to check uniqueness", req.Title))
	} else if existing != nil && (existingAssessment == nil || existing.ID != existingAssessment.ID) {
		errors = append(errors, *NewValidationError("title", "already exists for this creator", req.Title))
	}

	// QT-009: Due date must be in the future
	if req.DueDate != nil && req.DueDate.Before(time.Now()) {
		errors = append(errors, *NewValidationError("due_date", "must be in the future", req.DueDate))
	}

	return errors
}

func (v *ValidationService) validateAssessmentUpdateRules(ctx context.Context, req *UpdateAssessmentRequest, assessment *models.Assessment, userID uint) ValidationErrors {
	var errors ValidationErrors

	// Check if assessment has attempts for restricted updates
	hasAttempts, err := v.repo.Assessment().HasAttempts(ctx, assessment.ID)
	if err != nil {
		errors = append(errors, *NewValidationError("assessment", "failed to check existing attempts", assessment.ID))
		return errors
	}

	if hasAttempts {
		// Restricted fields when assessment has attempts
		if req.Duration != nil && *req.Duration != assessment.Duration {
			errors = append(errors, *NewValidationError("duration", "cannot be changed after students have started attempts", *req.Duration))
		}

		if req.PassingScore != nil && *req.PassingScore != assessment.PassingScore {
			errors = append(errors, *NewValidationError("passing_score", "cannot be changed after students have started attempts", *req.PassingScore))
		}

		if req.MaxAttempts != nil && *req.MaxAttempts < assessment.MaxAttempts {
			errors = append(errors, *NewValidationError("max_attempts", "cannot be decreased after students have started attempts", *req.MaxAttempts))
		}
	}

	// Title uniqueness check if being updated
	if req.Title != nil && *req.Title != assessment.Title {
		existing, err := v.repo.Assessment().GetByTitleAndCreator(ctx, *req.Title, assessment.CreatedBy)
		if err != nil && !repositories.IsNotFoundError(err) {
			errors = append(errors, *NewValidationError("title", "failed to check uniqueness", *req.Title))
		} else if existing != nil && existing.ID != assessment.ID {
			errors = append(errors, *NewValidationError("title", "already exists for this creator", *req.Title))
		}
	}

	// Due date validation
	if req.DueDate != nil && req.DueDate.Before(time.Now()) {
		errors = append(errors, *NewValidationError("due_date", "must be in the future", *req.DueDate))
	}

	return errors
}

func (v *ValidationService) validateAssessmentSettings(settings *AssessmentSettingsRequest) ValidationErrors {
	var errors ValidationErrors

	// Questions per page validation
	if settings.QuestionsPerPage != nil && (*settings.QuestionsPerPage < 1 || *settings.QuestionsPerPage > 50) {
		errors = append(errors, *NewValidationError("settings.questions_per_page", "must be between 1 and 50", *settings.QuestionsPerPage))
	}

	// Retake delay validation
	if settings.RetakeDelay != nil && (*settings.RetakeDelay < 0 || *settings.RetakeDelay > 1440) {
		errors = append(errors, *NewValidationError("settings.retake_delay", "must be between 0 and 1440 minutes", *settings.RetakeDelay))
	}

	// Font size adjustment validation
	if settings.FontSizeAdjustment != nil && (*settings.FontSizeAdjustment < -2 || *settings.FontSizeAdjustment > 2) {
		errors = append(errors, *NewValidationError("settings.font_size_adjustment", "must be between -2 and 2", *settings.FontSizeAdjustment))
	}

	// Logical consistency checks
	if settings.AllowRetake != nil && settings.RetakeDelay != nil {
		if !*settings.AllowRetake && *settings.RetakeDelay > 0 {
			errors = append(errors, *NewValidationError("settings.retake_delay", "should be 0 when retakes are not allowed", *settings.RetakeDelay))
		}
	}

	return errors
}

func (v *ValidationService) validateAssessmentQuestions(ctx context.Context, questions []AssessmentQuestionRequest, creatorID uint) ValidationErrors {
	var errors ValidationErrors

	// Check for duplicate orders
	orderMap := make(map[int]bool)
	for i, q := range questions {
		if orderMap[q.Order] {
			errors = append(errors, *NewValidationError(fmt.Sprintf("questions[%d].order", i), "duplicate order number", q.Order))
		}
		orderMap[q.Order] = true

		// Validate question exists and user has access
		question, err := v.repo.Question().GetByID(ctx, q.QuestionID)
		if err != nil {
			if repositories.IsNotFoundError(err) {
				errors = append(errors, *NewValidationError(fmt.Sprintf("questions[%d].question_id", i), "question not found", q.QuestionID))
			} else {
				errors = append(errors, *NewValidationError(fmt.Sprintf("questions[%d].question_id", i), "failed to validate question", q.QuestionID))
			}
			continue
		}

		// Check if user can access the question
		if question.CreatedBy != creatorID {
			// TODO: Add more sophisticated permission checking for shared questions
			userRole, err := v.getUserRole(ctx, creatorID)
			if err != nil || userRole != models.RoleAdmin {
				errors = append(errors, *NewValidationError(fmt.Sprintf("questions[%d].question_id", i), "access denied to question", q.QuestionID))
			}
		}

		// Validate points override if provided
		if q.Points != nil && (*q.Points < 1 || *q.Points > 100) {
			errors = append(errors, *NewValidationError(fmt.Sprintf("questions[%d].points", i), "must be between 1 and 100", *q.Points))
		}
	}

	return errors
}

// ===== QUESTION VALIDATION =====

func (v *ValidationService) ValidateQuestionCreate(ctx context.Context, req *CreateQuestionRequest, creatorID uint) ValidationErrors {
	var errors ValidationErrors

	// Basic field validation
	errors = append(errors, v.validateQuestionBasicFields(req.Text, req.Points, req.TimeLimit, req.Difficulty)...)

	// Category validation
	if req.CategoryID != nil {
		errors = append(errors, v.validateQuestionCategory(ctx, *req.CategoryID, creatorID)...)
	}

	// Content validation based on type
	errors = append(errors, v.validateQuestionTypeContent(req.Type, req.Content)...)

	// Tags validation
	errors = append(errors, v.validateQuestionTags(req.Tags)...)

	return errors
}

func (v *ValidationService) ValidateQuestionUpdate(ctx context.Context, req *UpdateQuestionRequest, question *models.Question, userID uint) ValidationErrors {
	var errors ValidationErrors

	// Basic field validation if being updated
	text := question.Text
	if req.Text != nil {
		text = *req.Text
	}

	points := question.Points
	if req.Points != nil {
		points = *req.Points
	}

	timeLimit := question.TimeLimit
	if req.TimeLimit != nil {
		timeLimit = req.TimeLimit
	}

	difficulty := question.Difficulty
	if req.Difficulty != nil {
		difficulty = *req.Difficulty
	}

	errors = append(errors, v.validateQuestionBasicFields(text, points, timeLimit, difficulty)...)

	// Category validation
	if req.CategoryID != nil {
		errors = append(errors, v.validateQuestionCategory(ctx, *req.CategoryID, userID)...)
	}

	// Content validation if being updated
	if req.Content != nil {
		errors = append(errors, v.validateQuestionTypeContent(question.Type, req.Content)...)
	}

	// Tags validation
	if req.Tags != nil {
		errors = append(errors, v.validateQuestionTags(req.Tags)...)
	}

	// Business rule: Check if question is in use and content is being modified
	if req.Content != nil {
		inUse, err := v.repo.Question().IsInUse(ctx, question.ID)
		if err != nil {
			errors = append(errors, *NewValidationError("question", "failed to check usage", question.ID))
		} else if inUse {
			errors = append(errors, *NewValidationError("content", "cannot modify content of question in active assessments", nil))
		}
	}

	return errors
}

func (v *ValidationService) validateQuestionBasicFields(text string, points int, timeLimit *int, difficulty models.DifficultyLevel) ValidationErrors {
	var errors ValidationErrors

	// Text validation
	if strings.TrimSpace(text) == "" {
		errors = append(errors, *NewValidationError("text", "cannot be empty", text))
	}

	if len(text) > 2000 {
		errors = append(errors, *NewValidationError("text", "cannot exceed 2000 characters", len(text)))
	}

	// Points validation
	if points < 1 || points > 100 {
		errors = append(errors, *NewValidationError("points", "must be between 1 and 100", points))
	}

	// Time limit validation
	if timeLimit != nil && (*timeLimit < 30 || *timeLimit > 3600) {
		errors = append(errors, *NewValidationError("time_limit", "must be between 30 and 3600 seconds", *timeLimit))
	}

	// Difficulty validation
	validDifficulties := map[models.DifficultyLevel]bool{
		models.DifficultyEasy:   true,
		models.DifficultyMedium: true,
		models.DifficultyHard:   true,
	}

	if !validDifficulties[difficulty] {
		errors = append(errors, *NewValidationError("difficulty", "must be Easy, Medium, or Hard", difficulty))
	}

	return errors
}

func (v *ValidationService) validateQuestionCategory(ctx context.Context, categoryID uint, userID uint) ValidationErrors {
	var errors ValidationErrors

	category, err := v.repo.QuestionCategory().GetByID(ctx, categoryID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			errors = append(errors, *NewValidationError("category_id", "category not found", categoryID))
		} else {
			errors = append(errors, *NewValidationError("category_id", "failed to validate category", categoryID))
		}
		return errors
	}

	// Check access permission
	userRole, err := v.getUserRole(ctx, userID)
	if err != nil {
		errors = append(errors, *NewValidationError("category_id", "failed to check permissions", categoryID))
		return errors
	}

	if userRole != models.RoleAdmin && category.CreatedBy != userID {
		errors = append(errors, *NewValidationError("category_id", "access denied to category", categoryID))
	}

	return errors
}

func (v *ValidationService) validateQuestionTags(tags []string) ValidationErrors {
	var errors ValidationErrors

	if len(tags) > 10 {
		errors = append(errors, *NewValidationError("tags", "cannot have more than 10 tags", len(tags)))
	}

	for i, tag := range tags {
		if strings.TrimSpace(tag) == "" {
			errors = append(errors, *NewValidationError(fmt.Sprintf("tags[%d]", i), "tag cannot be empty", tag))
		}

		if len(tag) > 50 {
			errors = append(errors, *NewValidationError(fmt.Sprintf("tags[%d]", i), "tag cannot exceed 50 characters", len(tag)))
		}
	}

	return errors
}

func (v *ValidationService) validateQuestionTypeContent(questionType models.QuestionType, content interface{}) ValidationErrors {
	// This delegates to the same validation logic used in QuestionService
	// We create a temporary question service instance for validation only
	tempService := &questionService{repo: v.repo}

	err := tempService.validateQuestionContent(questionType, content)
	if err != nil {
		if validationErrors, ok := err.(ValidationErrors); ok {
			return validationErrors
		}
		return ValidationErrors{*NewValidationError("content", err.Error(), content)}
	}

	return nil
}

// ===== ATTEMPT VALIDATION =====

func (v *ValidationService) ValidateAttemptStart(ctx context.Context, assessmentID uint, studentID uint) ValidationErrors {
	var errors ValidationErrors

	// Get assessment
	assessment, err := v.repo.Assessment().GetByID(ctx, assessmentID)
	if err != nil {
		errors = append(errors, *NewValidationError("assessment_id", "assessment not found", assessmentID))
		return errors
	}

	// Check assessment status
	if assessment.Status != models.StatusActive {
		errors = append(errors, *NewValidationError("assessment", "assessment is not active", assessment.Status))
	}

	// Check due date
	if assessment.DueDate != nil && time.Now().After(*assessment.DueDate) {
		errors = append(errors, *NewValidationError("assessment", "assessment has expired", assessment.DueDate))
	}

	// Check attempt limits
	attemptCount, err := v.repo.Attempt().GetStudentAttemptCount(ctx, assessmentID, studentID)
	if err != nil {
		errors = append(errors, *NewValidationError("attempts", "failed to check attempt count", studentID))
	} else if attemptCount >= assessment.MaxAttempts {
		errors = append(errors, *NewValidationError("attempts", "maximum attempts exceeded", attemptCount))
	}

	// Check for existing active attempt
	currentAttempt, err := v.repo.Attempt().GetCurrentAttempt(ctx, assessmentID, studentID)
	if err != nil && !repositories.IsNotFoundError(err) {
		errors = append(errors, *NewValidationError("attempts", "failed to check current attempt", studentID))
	} else if currentAttempt != nil && currentAttempt.Status == models.AttemptStatusInProgress {
		// Check if expired
		if currentAttempt.EndTime != nil && time.Now().After(*currentAttempt.EndTime) {
			// Allow start of new attempt
		} else {
			errors = append(errors, *NewValidationError("attempts", "active attempt already exists", currentAttempt.ID))
		}
	}

	return errors
}

func (v *ValidationService) ValidateAnswerSubmission(ctx context.Context, attemptID uint, req *SubmitAnswerRequest, studentID uint) ValidationErrors {
	var errors ValidationErrors

	// Get attempt
	attempt, err := v.repo.Attempt().GetByID(ctx, attemptID)
	if err != nil {
		errors = append(errors, *NewValidationError("attempt_id", "attempt not found", attemptID))
		return errors
	}

	// Verify ownership
	if attempt.StudentID != studentID {
		errors = append(errors, *NewValidationError("attempt", "access denied", attemptID))
		return errors
	}

	// Check attempt status
	if attempt.Status != models.AttemptStatusInProgress {
		errors = append(errors, *NewValidationError("attempt", "attempt is not active", attempt.Status))
	}

	// Check time limit
	if attempt.EndTime != nil && time.Now().After(*attempt.EndTime) {
		errors = append(errors, *NewValidationError("attempt", "time limit exceeded", attempt.EndTime))
	}

	// Validate question belongs to assessment
	questionInAssessment, err := v.repo.AssessmentQuestion().QuestionInAssessment(ctx, attempt.AssessmentID, req.QuestionID)
	if err != nil {
		errors = append(errors, *NewValidationError("question_id", "failed to validate question", req.QuestionID))
	} else if !questionInAssessment {
		errors = append(errors, *NewValidationError("question_id", "question not in assessment", req.QuestionID))
	}

	// Validate answer data format
	if !req.IsSkipped && req.AnswerData == nil {
		errors = append(errors, *NewValidationError("answer_data", "answer data required when not skipped", nil))
	}

	return errors
}

// ===== BUSINESS RULE VALIDATION =====

func (v *ValidationService) ValidateStatusTransition(ctx context.Context, currentStatus, newStatus models.AssessmentStatus, assessmentID uint) error {
	// Define allowed transitions
	allowedTransitions := map[models.AssessmentStatus][]models.AssessmentStatus{
		models.StatusDraft:    {models.StatusActive, models.StatusArchived},
		models.StatusActive:   {models.StatusExpired, models.StatusArchived},
		models.StatusExpired:  {models.StatusActive, models.StatusArchived},
		models.StatusArchived: {}, // No transitions allowed from archived
	}

	allowed := false
	for _, allowedStatus := range allowedTransitions[currentStatus] {
		if newStatus == allowedStatus {
			allowed = true
			break
		}
	}

	if !allowed {
		return NewBusinessRuleError(
			"QT-INVALID-STATUS-TRANSITION",
			fmt.Sprintf("Cannot transition from %s to %s", currentStatus, newStatus),
			map[string]interface{}{
				"current_status": currentStatus,
				"new_status":     newStatus,
				"assessment_id":  assessmentID,
			},
		)
	}

	// Additional validation for publishing
	if newStatus == models.StatusActive {
		return v.ValidateAssessmentReadyForPublish(ctx, assessmentID)
	}

	return nil
}

func (v *ValidationService) ValidateAssessmentReadyForPublish(ctx context.Context, assessmentID uint) error {
	// Must have at least one question
	questionCount, err := v.repo.AssessmentQuestion().GetQuestionCount(ctx, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to get question count: %w", err)
	}

	if questionCount == 0 {
		return NewBusinessRuleError(
			"QT-ASSESSMENT-NO-QUESTIONS",
			"Assessment must have at least one question before publishing",
			map[string]interface{}{
				"assessment_id": assessmentID,
			},
		)
	}

	return nil
}

func (v *ValidationService) ValidateDeletePermissions(ctx context.Context, resourceType string, resourceID uint, userID uint) error {
	switch resourceType {
	case "assessment":
		return v.validateAssessmentDeletion(ctx, resourceID, userID)
	case "question":
		return v.validateQuestionDeletion(ctx, resourceID, userID)
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

func (v *ValidationService) validateAssessmentDeletion(ctx context.Context, assessmentID uint, userID uint) error {
	// Check if assessment has attempts
	hasAttempts, err := v.repo.Assessment().HasAttempts(ctx, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to check attempts: %w", err)
	}

	if hasAttempts {
		// Check if user is admin (can override)
		userRole, err := v.getUserRole(ctx, userID)
		if err != nil {
			return err
		}

		if userRole != models.RoleAdmin {
			return NewBusinessRuleError(
				"QT-ASSESSMENT-HAS-ATTEMPTS",
				"Cannot delete assessment with existing attempts",
				map[string]interface{}{
					"assessment_id": assessmentID,
					"user_id":       userID,
				},
			)
		}
	}

	return nil
}

func (v *ValidationService) validateQuestionDeletion(ctx context.Context, questionID uint, userID uint) error {
	// Check if question is in use
	inUse, err := v.repo.Question().IsInUse(ctx, questionID)
	if err != nil {
		return fmt.Errorf("failed to check question usage: %w", err)
	}

	if inUse {
		// Check if user is admin (can override)
		userRole, err := v.getUserRole(ctx, userID)
		if err != nil {
			return err
		}

		if userRole != models.RoleAdmin {
			return NewBusinessRuleError(
				"QT-QUESTION-IN-USE",
				"Cannot delete question that is in use by assessments",
				map[string]interface{}{
					"question_id": questionID,
					"user_id":     userID,
				},
			)
		}
	}

	return nil
}

// ===== HELPER FUNCTIONS =====

func (v *ValidationService) getUserRole(ctx context.Context, userID uint) (models.UserRole, error) {
	user, err := v.repo.User().GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}
	return user.Role, nil
}
