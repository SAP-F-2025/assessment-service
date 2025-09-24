package validator

import (
	"reflect"
	"strings"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/go-playground/validator/v10"
)

// Validator is the main validator instance that combines all validation types
type Validator struct {
	structValidator   *validator.Validate
	businessValidator *BusinessValidator
	questionValidator *QuestionValidator
}

// New creates a new centralized validator instance
func New() *Validator {
	structValidator := validator.New()

	// Register all custom validators once
	registerCustomValidators(structValidator)

	return &Validator{
		structValidator:   structValidator,
		businessValidator: NewBusinessValidator(),
		questionValidator: NewQuestionValidator(),
	}
}

// ValidateStruct validates struct tags only
func (v *Validator) ValidateStruct(s interface{}) error {
	return v.structValidator.Struct(s)
}

// ValidateBusiness validates business rules only
func (v *Validator) ValidateBusiness(s interface{}) ValidationErrors {
	return v.businessValidator.Validate(s)
}

// Validate performs complete validation (struct + business rules)
func (v *Validator) Validate(s interface{}) error {
	// First validate struct tags
	if err := v.ValidateStruct(s); err != nil {
		return err
	}

	// Then validate business rules
	if errors := v.ValidateBusiness(s); len(errors) > 0 {
		return errors
	}

	return nil
}

// Question returns the question validator
func (v *Validator) Question() *QuestionValidator {
	return v.questionValidator
}

// Business returns the business validator
func (v *Validator) Business() *BusinessValidator {
	return v.businessValidator
}

// GetBusinessValidator returns the business validator (compatibility method)
func (v *Validator) GetBusinessValidator() *BusinessValidator {
	return v.businessValidator
}

func (v *Validator) GetQuestionValidator() *QuestionValidator {
	return v.questionValidator
}

// registerCustomValidators registers all custom validation functions
func registerCustomValidators(validate *validator.Validate) {
	// Question type validation
	validate.RegisterValidation("question_type", validateQuestionType)

	// Difficulty level validation
	validate.RegisterValidation("difficulty_level", validateDifficultyLevel)

	// User role validation
	validate.RegisterValidation("user_role", validateUserRole)

	// Assessment status validation
	validate.RegisterValidation("assessment_status", validateAssessmentStatus)

	// Custom tag name function for better error messages
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}

// Custom validation functions
func validateQuestionType(fl validator.FieldLevel) bool {
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

func validateDifficultyLevel(fl validator.FieldLevel) bool {
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

func validateUserRole(fl validator.FieldLevel) bool {
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

func validateAssessmentStatus(fl validator.FieldLevel) bool {
	validStatuses := []models.AssessmentStatus{
		models.StatusDraft,
		models.StatusActive,
		models.StatusExpired,
		models.StatusArchived,
	}

	value := fl.Field().String()
	for _, validStatus := range validStatuses {
		if string(validStatus) == value {
			return true
		}
	}
	return false
}
