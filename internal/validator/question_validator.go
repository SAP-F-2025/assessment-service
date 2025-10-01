package validator

import (
	"encoding/json"
	"fmt"

	"github.com/SAP-F-2025/assessment-service/internal/models"
)

// QuestionValidator handles question-specific validation
type QuestionValidator struct{}

// NewQuestionValidator creates a new question validator
func NewQuestionValidator() *QuestionValidator {
	return &QuestionValidator{}
}

// ValidateContent validates question content based on question type
func (v *QuestionValidator) ValidateContent(questionType models.QuestionType, content interface{}) error {
	if content == nil {
		return fmt.Errorf("content cannot be nil")
	}

	contentBytes, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}

	switch questionType {
	case models.MultipleChoice:
		return v.validateMultipleChoiceContent(contentBytes)
	case models.TrueFalse:
		return v.validateTrueFalseContent(contentBytes)
	case models.Essay:
		return v.validateEssayContent(contentBytes)
	case models.FillInBlank:
		return v.validateFillBlankContent(contentBytes)
	case models.Matching:
		return v.validateMatchingContent(contentBytes)
	case models.Ordering:
		return v.validateOrderingContent(contentBytes)
	case models.ShortAnswer:
		return v.validateShortAnswerContent(contentBytes)
	default:
		return fmt.Errorf("unsupported question type: %s", questionType)
	}
}

// ValidateQuestion validates a complete question object
func (v *QuestionValidator) ValidateQuestion(question *models.Question) error {
	if question.Text == "" {
		return fmt.Errorf("question text is required")
	}

	if question.Points < 1 || question.Points > 100 {
		return fmt.Errorf("question points must be between 1 and 100")
	}

	return v.ValidateContent(question.Type, question.Content)
}

// ValidateBatch validates multiple questions
func (v *QuestionValidator) ValidateBatch(questions []*models.Question) error {
	if len(questions) == 0 {
		return fmt.Errorf("question batch cannot be empty")
	}

	for i, question := range questions {
		if err := v.ValidateQuestion(question); err != nil {
			return fmt.Errorf("validation failed for question %d: %w", i+1, err)
		}
	}

	return nil
}

// ValidateUsage validates question usage constraints
func (v *QuestionValidator) ValidateUsage(isUsedInAssessments bool, operation string) error {
	if isUsedInAssessments && operation == "delete" {
		return fmt.Errorf("cannot delete question: it is used in active assessments")
	}
	return nil
}

// ValidateText validates question text uniqueness
func (v *QuestionValidator) ValidateText(text string, exists bool) error {
	if text == "" {
		return fmt.Errorf("question text cannot be empty")
	}
	if exists {
		return fmt.Errorf("question with this text already exists for this creator")
	}
	return nil
}

// Private validation methods for each question type

func (v *QuestionValidator) validateMultipleChoiceContent(contentBytes []byte) error {
	var content models.MultipleChoiceContent
	if err := json.Unmarshal(contentBytes, &content); err != nil {
		return fmt.Errorf("invalid multiple choice content: %w", err)
	}

	if len(content.Options) < 2 {
		return fmt.Errorf("must have at least 2 options")
	}

	if len(content.Options) > 10 {
		return fmt.Errorf("cannot have more than 10 options")
	}

	if len(content.CorrectAnswers) == 0 {
		return fmt.Errorf("must have at least 1 correct answer")
	}

	// Validate option IDs and text
	optionIDs := make(map[string]bool)
	for _, option := range content.Options {
		if option.Text == "" {
			return fmt.Errorf("option text cannot be empty")
		}
		optionIDs[option.ID] = true
	}

	// Validate correct answers exist in options
	for _, correctID := range content.CorrectAnswers {
		if !optionIDs[correctID] {
			return fmt.Errorf("correct answer ID '%s' does not match any option", correctID)
		}
	}

	// Multiple correct answers validation
	if len(content.CorrectAnswers) > 1 && !content.MultipleCorrect {
		return fmt.Errorf("multiple correct answers require MultipleCorrect to be true")
	}

	return nil
}

func (v *QuestionValidator) validateTrueFalseContent(contentBytes []byte) error {
	var content models.TrueFalseContent
	if err := json.Unmarshal(contentBytes, &content); err != nil {
		return fmt.Errorf("invalid true/false content: %w", err)
	}
	return nil
}

func (v *QuestionValidator) validateEssayContent(contentBytes []byte) error {
	var content models.EssayContent
	if err := json.Unmarshal(contentBytes, &content); err != nil {
		return fmt.Errorf("invalid essay content: %w", err)
	}

	if content.MinWords != nil && content.MaxWords != nil && *content.MinWords > *content.MaxWords {
		return fmt.Errorf("minimum word count cannot be greater than maximum")
	}

	if content.MinWords != nil && *content.MinWords < 0 {
		return fmt.Errorf("minimum word count cannot be negative")
	}

	if content.MaxWords != nil && *content.MaxWords < 0 {
		return fmt.Errorf("maximum word count cannot be negative")
	}

	return nil
}

func (v *QuestionValidator) validateFillBlankContent(contentBytes []byte) error {
	var content models.FillBlankContent
	if err := json.Unmarshal(contentBytes, &content); err != nil {
		return fmt.Errorf("invalid fill-in-blank content: %w", err)
	}

	if content.Template == "" {
		return fmt.Errorf("template is required")
	}

	if len(content.Blanks) == 0 {
		return fmt.Errorf("must have at least 1 blank")
	}

	for blankID, blankDef := range content.Blanks {
		if len(blankDef.AcceptedAnswers) == 0 {
			return fmt.Errorf("blank '%s' must have at least 1 accepted answer", blankID)
		}
		if blankDef.Points < 0 {
			return fmt.Errorf("blank '%s' points cannot be negative", blankID)
		}
	}

	return nil
}

func (v *QuestionValidator) validateMatchingContent(contentBytes []byte) error {
	var content models.MatchingContent
	if err := json.Unmarshal(contentBytes, &content); err != nil {
		return fmt.Errorf("invalid matching content: %w", err)
	}

	if len(content.LeftItems) < 2 {
		return fmt.Errorf("must have at least 2 left items")
	}

	if len(content.RightItems) < 2 {
		return fmt.Errorf("must have at least 2 right items")
	}

	if len(content.LeftItems) > 10 || len(content.RightItems) > 10 {
		return fmt.Errorf("cannot have more than 10 items on each side")
	}

	if len(content.CorrectPairs) == 0 {
		return fmt.Errorf("must have at least 1 correct pair")
	}

	// Validate item IDs and text
	leftIDs := make(map[string]bool)
	rightIDs := make(map[string]bool)

	for _, item := range content.LeftItems {
		if item.ID == "" || item.Text == "" {
			return fmt.Errorf("left items must have both ID and text")
		}
		leftIDs[item.ID] = true
	}

	for _, item := range content.RightItems {
		if item.ID == "" || item.Text == "" {
			return fmt.Errorf("right items must have both ID and text")
		}
		rightIDs[item.ID] = true
	}

	// Validate correct pairs
	for _, pair := range content.CorrectPairs {
		if !leftIDs[pair.LeftID] {
			return fmt.Errorf("correct pair references non-existent left item: %s", pair.LeftID)
		}
		if !rightIDs[pair.RightID] {
			return fmt.Errorf("correct pair references non-existent right item: %s", pair.RightID)
		}
	}

	return nil
}

func (v *QuestionValidator) validateOrderingContent(contentBytes []byte) error {
	var content models.OrderingContent
	if err := json.Unmarshal(contentBytes, &content); err != nil {
		return fmt.Errorf("invalid ordering content: %w", err)
	}

	if len(content.Items) < 2 {
		return fmt.Errorf("must have at least 2 items")
	}

	if len(content.Items) > 10 {
		return fmt.Errorf("cannot have more than 10 items")
	}

	if len(content.CorrectOrder) != len(content.Items) {
		return fmt.Errorf("correct order must include all items exactly once")
	}

	// Validate item IDs and text
	itemIDs := make(map[string]bool)
	for _, item := range content.Items {
		if item.ID == "" || item.Text == "" {
			return fmt.Errorf("items must have both ID and text")
		}
		itemIDs[item.ID] = true
	}

	// Validate correct order
	orderIDs := make(map[string]bool)
	for _, orderID := range content.CorrectOrder {
		if !itemIDs[orderID] {
			return fmt.Errorf("correct order references non-existent item: %s", orderID)
		}
		if orderIDs[orderID] {
			return fmt.Errorf("correct order contains duplicate item: %s", orderID)
		}
		orderIDs[orderID] = true
	}

	return nil
}

func (v *QuestionValidator) validateShortAnswerContent(contentBytes []byte) error {
	var content models.ShortAnswerContent
	if err := json.Unmarshal(contentBytes, &content); err != nil {
		return fmt.Errorf("invalid short answer content: %w", err)
	}

	if len(content.AcceptedAnswers) == 0 {
		return fmt.Errorf("must have at least 1 accepted answer")
	}

	if content.MaxLength < 1 {
		return fmt.Errorf("max length must be at least 1")
	}

	if content.MaxLength > 500 {
		return fmt.Errorf("max length cannot exceed 500 characters")
	}

	for i, answer := range content.AcceptedAnswers {
		if len(answer) > content.MaxLength {
			return fmt.Errorf("accepted answer %d exceeds max length of %d", i+1, content.MaxLength)
		}
		if answer == "" {
			return fmt.Errorf("accepted answer %d cannot be empty", i+1)
		}
	}

	return nil
}
