package utils

import (
	"reflect"
	"strings"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/go-playground/validator/v10"
)

// Validator wraps the go-playground validator with business rules
type Validator struct {
	businessValidator *BusinessValidator
	structValidator   *validator.Validate
}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	structValidator := validator.New()
	RegisterCustomValidators(structValidator)

	return &Validator{
		businessValidator: NewBusinessValidator(),
		structValidator:   structValidator,
	}
}

// Validate validates a struct using both struct tags and business rules
func (v *Validator) Validate(s interface{}) error {
	// First run struct validation
	if err := v.structValidator.Struct(s); err != nil {
		return err
	}

	// Then run business validation
	if errors := v.businessValidator.Validate(s); len(errors) > 0 {
		return errors
	}

	return nil
}

// ValidateStruct validates only struct tags
func (v *Validator) ValidateStruct(s interface{}) error {
	return v.structValidator.Struct(s)
}

// ValidateBusiness validates only business rules
func (v *Validator) ValidateBusiness(s interface{}) ValidationErrors {
	return v.businessValidator.Validate(s)
}

// GetBusinessValidator returns the business validator
func (v *Validator) GetBusinessValidator() *BusinessValidator {
	return v.businessValidator
}

// Custom validation functions

func ValidateQuestionType(fl validator.FieldLevel) bool {
	validTypes := []models.QuestionType{
		models.MultipleChoice,
		models.TrueFalse,
		models.Essay,
		models.FillInBlank,
		models.Matching,
		models.Ordering,
		models.ShortAnswer,
	}

	value := fl.Field().String()
	for _, validType := range validTypes {
		if string(validType) == value {
			return true
		}
	}
	return false
}

func ValidateDifficultyLevel(fl validator.FieldLevel) bool {
	validLevels := []models.DifficultyLevel{
		models.DifficultyEasy,
		models.DifficultyMedium,
		models.DifficultyHard,
	}

	value := fl.Field().String()
	for _, validLevel := range validLevels {
		if string(validLevel) == value {
			return true
		}
	}
	return false
}

func ValidateUserRole(fl validator.FieldLevel) bool {
	validRoles := []models.UserRole{
		models.RoleStudent,
		models.RoleTeacher,
		models.RoleProctor,
		models.RoleAdmin,
	}

	value := fl.Field().String()
	for _, validRole := range validRoles {
		if string(validRole) == value {
			return true
		}
	}
	return false
}

// RegisterCustomValidators registers all custom validators
func RegisterCustomValidators(validate *validator.Validate) {
	validate.RegisterValidation("question_type", ValidateQuestionType)
	validate.RegisterValidation("difficulty_level", ValidateDifficultyLevel)
	validate.RegisterValidation("user_role", ValidateUserRole)

	// Register custom tag name function for better error messages
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}
