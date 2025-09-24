package validator

import (
	"github.com/SAP-F-2025/assessment-service/internal/errors"
)

// Use shared validation errors from errors package
type ValidationError = errors.ValidationError
type ValidationErrors = errors.ValidationErrors

// ToValidationErrors converts validator.ValidationErrors to our custom type
func ToValidationErrors(err error) ValidationErrors {
	return errors.ToValidationErrors(err)
}
