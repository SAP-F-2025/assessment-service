package repositories

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

// IsNotFoundError checks if the error represents a "not found" condition.
// This checks both the wrapped gorm.ErrRecordNotFound and error messages
// containing "not found" (for cases where the error is not properly wrapped).
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Check for wrapped gorm error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true
	}
	// Fallback: check error message for "not found" pattern
	// This handles cases where repositories use fmt.Errorf without %w
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}
