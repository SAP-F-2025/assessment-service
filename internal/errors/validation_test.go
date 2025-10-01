package errors

import (
	"testing"
)

func TestValidationError(t *testing.T) {
	// Test NewValidationError
	err := NewValidationError("test_field", "test message", "test_value")

	if err.Field != "test_field" {
		t.Errorf("Expected field to be 'test_field', got '%s'", err.Field)
	}

	if err.Message != "test message" {
		t.Errorf("Expected message to be 'test message', got '%s'", err.Message)
	}

	if err.Value != "test_value" {
		t.Errorf("Expected value to be 'test_value', got '%v'", err.Value)
	}

	// Test Error method
	expected := "validation error on field 'test_field': test message"
	if err.Error() != expected {
		t.Errorf("Expected error message to be '%s', got '%s'", expected, err.Error())
	}
}

func TestValidationErrors(t *testing.T) {
	// Test empty ValidationErrors
	var errs ValidationErrors
	if errs.Error() != "validation failed" {
		t.Errorf("Expected 'validation failed' for empty errors, got '%s'", errs.Error())
	}

	// Test single ValidationError
	errs = append(errs, *NewValidationError("field1", "message1", nil))
	expected := "validation failed: field1 message1"
	if errs.Error() != expected {
		t.Errorf("Expected '%s' for single error, got '%s'", expected, errs.Error())
	}

	// Test multiple ValidationErrors
	errs = append(errs, *NewValidationError("field2", "message2", nil))
	expected = "validation failed: 2 field errors"
	if errs.Error() != expected {
		t.Errorf("Expected '%s' for multiple errors, got '%s'", expected, errs.Error())
	}
}

func TestNewValidationErrorWithRule(t *testing.T) {
	err := NewValidationErrorWithRule("test_field", "test message", "required", "test_value")

	if err.Rule != "required" {
		t.Errorf("Expected rule to be 'required', got '%s'", err.Rule)
	}

	if err.Field != "test_field" {
		t.Errorf("Expected field to be 'test_field', got '%s'", err.Field)
	}
}
