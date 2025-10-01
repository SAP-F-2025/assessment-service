package errors

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// ValidationError represents a single validation error
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
	Rule    string      `json:"rule,omitempty"`
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return "validation failed"
	}
	if len(ve) == 1 {
		return fmt.Sprintf("validation failed: %s %s", ve[0].Field, ve[0].Message)
	}
	return fmt.Sprintf("validation failed: %d field errors", len(ve))
}

func (pe *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", pe.Field, pe.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string, value interface{}) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	}
}

// NewValidationErrorWithRule creates a new validation error with rule
func NewValidationErrorWithRule(field, message, rule string, value interface{}) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
		Rule:    rule,
	}
}

// ToValidationErrors converts validator.ValidationErrors to our custom type
func ToValidationErrors(err error) ValidationErrors {
	var errors ValidationErrors

	if validatorErr, ok := err.(validator.ValidationErrors); ok {
		for _, err := range validatorErr {
			errors = append(errors, ValidationError{
				Field:   err.Field(),
				Message: getErrorMessage(err),
				Value:   err.Value(),
				Rule:    err.Tag(),
			})
		}
	}

	return errors
}

// getErrorMessage returns user-friendly error messages
func getErrorMessage(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return "is required"
	case "min":
		return fmt.Sprintf("must be at least %s", err.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", err.Param())
	case "len":
		return fmt.Sprintf("must be exactly %s characters", err.Param())
	case "email":
		return "must be a valid email address"
	case "uuid":
		return "must be a valid UUID"
	case "numeric":
		return "must be a number"
	case "alpha":
		return "must contain only letters"
	case "alphanum":
		return "must contain only letters and numbers"
	case "oneof":
		return fmt.Sprintf("must be one of: %s", err.Param())

	// Custom validators
	case "question_type":
		return "must be a valid question type (multiple_choice, true_false, essay, fill_blank, matching, ordering, short_answer)"
	case "difficulty_level":
		return "must be Easy, Medium, or Hard"
	case "user_role":
		return "must be a valid user role (student, teacher, proctor, admin)"
	case "assessment_status":
		return "must be a valid assessment status (draft, active, expired, archived)"

	// Business rule validators
	case "assessment_duration":
		return "must be between 5 and 300 minutes"
	case "passing_score":
		return "must be between 0 and 100"
	case "max_attempts":
		return "must be between 1 and 10"
	case "assessment_title":
		return "must be between 1 and 200 characters"
	case "assessment_description":
		return "must not exceed 1000 characters"
	case "future_date":
		return "must be in the future"
	case "points_range":
		return "must be between 1 and 100"
	case "time_limit":
		return "must be between 30 and 3600 seconds"

	default:
		return fmt.Sprintf("validation failed for rule '%s'", err.Tag())
	}
}
