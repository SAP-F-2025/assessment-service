package services

import (
	"errors"
	"fmt"

	apperrors "github.com/SAP-F-2025/assessment-service/internal/errors"
)

// ===== COMMON SERVICE ERRORS =====

var (
	// Generic errors
	ErrNotFound         = errors.New("resource not found")
	ErrUnauthorized     = errors.New("unauthorized access")
	ErrForbidden        = errors.New("forbidden - insufficient permissions")
	ErrValidationFailed = errors.New("validation failed")
	ErrInternalError    = errors.New("internal server error")
	ErrBadRequest       = errors.New("bad request")
	ErrConflict         = errors.New("resource conflict")

	// Assessment specific errors
	ErrAssessmentNotFound       = errors.New("assessment not found")
	ErrAssessmentAccessDenied   = errors.New("access denied to assessment")
	ErrAssessmentNotEditable    = errors.New("assessment cannot be edited in current status")
	ErrAssessmentNotDeletable   = errors.New("assessment cannot be deleted - has existing attempts")
	ErrAssessmentInvalidStatus  = errors.New("invalid assessment status transition")
	ErrAssessmentDuplicateTitle = errors.New("assessment title already exists for this user")
	ErrAssessmentExpired        = errors.New("assessment has expired")
	ErrAssessmentNotPublished   = errors.New("assessment is not published")

	// Question specific errors
	ErrQuestionNotFound       = errors.New("question not found")
	ErrQuestionAccessDenied   = errors.New("access denied to question")
	ErrQuestionInvalidType    = errors.New("invalid question type")
	ErrQuestionInvalidContent = errors.New("invalid question content for type")
	ErrQuestionNotDeletable   = errors.New("question cannot be deleted - in use by assessments")
	ErrQuestionDuplicateOrder = errors.New("question order already exists in assessment")

	// Question Bank specific errors
	ErrQuestionBankNotFound      = errors.New("question bank not found")
	ErrQuestionBankAccessDenied  = errors.New("access denied to question bank")
	ErrQuestionBankNotDeletable  = errors.New("question bank cannot be deleted - has existing questions")
	ErrQuestionBankDuplicateName = errors.New("question bank name already exists for this user")
	ErrQuestionBankShareExists   = errors.New("question bank already shared with this user")
	ErrQuestionBankNotShared     = errors.New("question bank is not shared with this user")

	// Attempt specific errors
	ErrAttemptNotFound         = errors.New("attempt not found")
	ErrAttemptAccessDenied     = errors.New("access denied to attempt")
	ErrAttemptNotActive        = errors.New("attempt is not active")
	ErrAttemptAlreadySubmitted = errors.New("attempt already submitted")
	ErrAttemptLimitExceeded    = errors.New("maximum attempts exceeded")
	ErrAttemptTimeExpired      = errors.New("attempt time has expired")
	ErrAttemptNotStarted       = errors.New("attempt not started")
	ErrAttemptCannotStart      = errors.New("cannot start new attempt")

	// Grading specific errors
	ErrGradingNotAllowed       = errors.New("grading not allowed for this question type")
	ErrGradingAlreadyCompleted = errors.New("answer already graded")
	ErrGradingInvalidScore     = errors.New("invalid score value")
	ErrGradingPermissionDenied = errors.New("permission denied for grading")

	// User/Permission errors
	ErrUserNotFound            = errors.New("user not found")
	ErrInvalidRole             = errors.New("invalid user role")
	ErrInsufficientPermissions = errors.New("insufficient permissions")
)

// ===== CUSTOM ERROR TYPES =====

// Use shared validation errors from errors package
type ValidationError = apperrors.ValidationError
type ValidationErrors = apperrors.ValidationErrors

type BusinessRuleError struct {
	Rule    string                 `json:"rule"`
	Message string                 `json:"message"`
	Context map[string]interface{} `json:"context,omitempty"`
}

func (bre *BusinessRuleError) Error() string {
	return fmt.Sprintf("business rule violation (%s): %s", bre.Rule, bre.Message)
}

type PermissionError struct {
	UserID     string `json:"user_id"`
	ResourceID uint   `json:"resource_id"`
	Resource   string `json:"resource"`
	Action     string `json:"action"`
	Reason     string `json:"reason"`
}

func (pe *PermissionError) Error() string {
	return fmt.Sprintf("permission denied: user %s cannot %s %s %d - %s",
		pe.UserID, pe.Action, pe.Resource, pe.ResourceID, pe.Reason)
}

// ===== ERROR HELPERS =====

// NewValidationError creates a new validation error using the shared type
func NewValidationError(field, message string, value interface{}) *ValidationError {
	return apperrors.NewValidationError(field, message, value)
}

func NewBusinessRuleError(rule, message string, context map[string]interface{}) *BusinessRuleError {
	return &BusinessRuleError{
		Rule:    rule,
		Message: message,
		Context: context,
	}
}

func NewPermissionError(userID string, resourceID uint, resource, action, reason string) *PermissionError {
	return &PermissionError{
		UserID:     userID,
		ResourceID: resourceID,
		Resource:   resource,
		Action:     action,
		Reason:     reason,
	}
}

// IsNotFound checks if error represents a "not found" condition
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) ||
		errors.Is(err, ErrAssessmentNotFound) ||
		errors.Is(err, ErrQuestionNotFound) ||
		errors.Is(err, ErrAttemptNotFound) ||
		errors.Is(err, ErrUserNotFound)
}

// IsUnauthorized checks if error represents an "unauthorized" condition
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized) ||
		errors.Is(err, ErrForbidden) ||
		errors.Is(err, ErrAssessmentAccessDenied) ||
		errors.Is(err, ErrQuestionAccessDenied) ||
		errors.Is(err, ErrAttemptAccessDenied) ||
		errors.Is(err, ErrInsufficientPermissions)
}

// IsValidation checks if error represents a validation failure
func IsValidation(err error) bool {
	if errors.Is(err, ErrValidationFailed) {
		return true
	}
	var ve apperrors.ValidationErrors
	return errors.As(err, &ve)
}

// IsBusinessRule checks if error represents a business rule violation
func IsBusinessRule(err error) bool {
	var bre *BusinessRuleError
	return errors.As(err, &bre)
}

// IsConflict checks if error represents a resource conflict
func IsConflict(err error) bool {
	return errors.Is(err, ErrConflict) ||
		errors.Is(err, ErrAssessmentNotDeletable) ||
		errors.Is(err, ErrAssessmentDuplicateTitle) ||
		errors.Is(err, ErrQuestionNotDeletable) ||
		errors.Is(err, ErrAttemptAlreadySubmitted) ||
		errors.Is(err, ErrAttemptLimitExceeded) ||
		errors.Is(err, ErrGradingAlreadyCompleted)
}
