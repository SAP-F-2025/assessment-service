package utils

import (
	"encoding/json"
	"fmt"

	"github.com/SAP-F-2025/assessment-service/internal/models"
)

// QuestionValidator handles validation logic for questions
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

	// Convert content to JSON for validation
	contentBytes, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}

	// Type-specific validation
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
	// Basic field validation
	if question.Text == "" {
		return fmt.Errorf("question text is required")
	}

	if question.Points < 1 || question.Points > 100 {
		return fmt.Errorf("question points must be between 1 and 100")
	}

	// Validate content
	return v.ValidateContent(question.Type, question.Content)
}

// ValidateQuestionBatch validates a batch of questions
func (v *QuestionValidator) ValidateQuestionBatch(questions []*models.Question) error {
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

// Business rule validation methods
func (v *QuestionValidator) ValidateQuestionUsage(isUsedInAssessments bool, operation string) error {
	if isUsedInAssessments && operation == "delete" {
		return fmt.Errorf("cannot delete question: it is used in active assessments")
	}
	return nil
}

func (v *QuestionValidator) ValidateQuestionText(text string, exists bool) error {
	if text == "" {
		return fmt.Errorf("question text cannot be empty")
	}
	if exists {
		return fmt.Errorf("question with this text already exists for this creator")
	}
	return nil
}

// ===== PRIVATE VALIDATION METHODS =====

func (v *QuestionValidator) validateMultipleChoiceContent(contentBytes []byte) error {
	var mcContent models.MultipleChoiceContent
	if err := json.Unmarshal(contentBytes, &mcContent); err != nil {
		return fmt.Errorf("invalid multiple choice content structure: %w", err)
	}

	if len(mcContent.Options) < 2 {
		return fmt.Errorf("multiple choice questions must have at least 2 options")
	}

	if len(mcContent.Options) > 10 {
		return fmt.Errorf("multiple choice questions cannot have more than 10 options")
	}

	if len(mcContent.CorrectAnswers) == 0 {
		return fmt.Errorf("multiple choice questions must have at least 1 correct answer")
	}

	// Validate that all correct answers exist in options
	optionIDs := make(map[string]bool)
	for _, option := range mcContent.Options {
		if option.Text == "" {
			return fmt.Errorf("option text cannot be empty")
		}
		optionIDs[option.ID] = true
	}

	for _, correctID := range mcContent.CorrectAnswers {
		if !optionIDs[correctID] {
			return fmt.Errorf("correct answer ID '%s' does not match any option", correctID)
		}
	}

	// If multiple correct answers, ensure MultipleCorrect is true
	if len(mcContent.CorrectAnswers) > 1 && !mcContent.MultipleCorrect {
		return fmt.Errorf("multiple correct answers require MultipleCorrect to be true")
	}

	return nil
}

func (v *QuestionValidator) validateTrueFalseContent(contentBytes []byte) error {
	var tfContent models.TrueFalseContent
	if err := json.Unmarshal(contentBytes, &tfContent); err != nil {
		return fmt.Errorf("invalid true/false content structure: %w", err)
	}

	// True/False content is simple, just ensure it unmarshals correctly
	return nil
}

func (v *QuestionValidator) validateEssayContent(contentBytes []byte) error {
	var essayContent models.EssayContent
	if err := json.Unmarshal(contentBytes, &essayContent); err != nil {
		return fmt.Errorf("invalid essay content structure: %w", err)
	}

	if essayContent.MinWords != nil && essayContent.MaxWords != nil && *essayContent.MinWords > *essayContent.MaxWords {
		return fmt.Errorf("minimum word count cannot be greater than maximum word count")
	}

	if essayContent.MinWords != nil && *essayContent.MinWords < 0 {
		return fmt.Errorf("minimum word count cannot be negative")
	}

	if essayContent.MaxWords != nil && *essayContent.MaxWords < 0 {
		return fmt.Errorf("maximum word count cannot be negative")
	}

	return nil
}

func (v *QuestionValidator) validateFillBlankContent(contentBytes []byte) error {
	var fillContent models.FillBlankContent
	if err := json.Unmarshal(contentBytes, &fillContent); err != nil {
		return fmt.Errorf("invalid fill-in-blank content structure: %w", err)
	}

	if fillContent.Template == "" {
		return fmt.Errorf("template is required for fill-in-blank questions")
	}

	if len(fillContent.Blanks) == 0 {
		return fmt.Errorf("fill-in-blank questions must have at least 1 blank")
	}

	// Validate each blank definition
	for blankID, blankDef := range fillContent.Blanks {
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
	var matchContent models.MatchingContent
	if err := json.Unmarshal(contentBytes, &matchContent); err != nil {
		return fmt.Errorf("invalid matching content structure: %w", err)
	}

	if len(matchContent.LeftItems) < 2 {
		return fmt.Errorf("matching questions must have at least 2 left items")
	}

	if len(matchContent.RightItems) < 2 {
		return fmt.Errorf("matching questions must have at least 2 right items")
	}

	if len(matchContent.LeftItems) > 10 || len(matchContent.RightItems) > 10 {
		return fmt.Errorf("matching questions cannot have more than 10 items on each side")
	}

	if len(matchContent.CorrectPairs) == 0 {
		return fmt.Errorf("matching questions must have at least 1 correct pair")
	}

	// Validate item IDs and text
	leftIDs := make(map[string]bool)
	rightIDs := make(map[string]bool)

	for _, item := range matchContent.LeftItems {
		if item.ID == "" || item.Text == "" {
			return fmt.Errorf("left items must have both ID and text")
		}
		leftIDs[item.ID] = true
	}

	for _, item := range matchContent.RightItems {
		if item.ID == "" || item.Text == "" {
			return fmt.Errorf("right items must have both ID and text")
		}
		rightIDs[item.ID] = true
	}

	// Validate correct pairs reference existing items
	for _, pair := range matchContent.CorrectPairs {
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
	var orderContent models.OrderingContent
	if err := json.Unmarshal(contentBytes, &orderContent); err != nil {
		return fmt.Errorf("invalid ordering content structure: %w", err)
	}

	if len(orderContent.Items) < 2 {
		return fmt.Errorf("ordering questions must have at least 2 items")
	}

	if len(orderContent.Items) > 10 {
		return fmt.Errorf("ordering questions cannot have more than 10 items")
	}

	if len(orderContent.CorrectOrder) != len(orderContent.Items) {
		return fmt.Errorf("correct order must include all items exactly once")
	}

	// Validate item IDs and text
	itemIDs := make(map[string]bool)
	for _, item := range orderContent.Items {
		if item.ID == "" || item.Text == "" {
			return fmt.Errorf("items must have both ID and text")
		}
		itemIDs[item.ID] = true
	}

	// Validate correct order references all items
	orderIDs := make(map[string]bool)
	for _, orderID := range orderContent.CorrectOrder {
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
	var shortContent models.ShortAnswerContent
	if err := json.Unmarshal(contentBytes, &shortContent); err != nil {
		return fmt.Errorf("invalid short answer content structure: %w", err)
	}

	if len(shortContent.AcceptedAnswers) == 0 {
		return fmt.Errorf("short answer questions must have at least 1 accepted answer")
	}

	if shortContent.MaxLength < 1 {
		return fmt.Errorf("max length must be at least 1")
	}

	if shortContent.MaxLength > 500 {
		return fmt.Errorf("max length cannot exceed 500 characters")
	}

	// Validate that accepted answers don't exceed max length
	for i, answer := range shortContent.AcceptedAnswers {
		if len(answer) > shortContent.MaxLength {
			return fmt.Errorf("accepted answer %d exceeds max length of %d", i+1, shortContent.MaxLength)
		}
		if answer == "" {
			return fmt.Errorf("accepted answer %d cannot be empty", i+1)
		}
	}

	return nil
}
