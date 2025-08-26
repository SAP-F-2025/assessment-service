package utils

import (
	"reflect"
	"strings"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/go-playground/validator/v10"
)

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
