package models

import "time"

type ImportSummary struct {
	TotalRows        int                     `json:"total_rows"`
	ProcessedRows    int                     `json:"processed_rows"`
	SuccessCount     int                     `json:"success_count"`
	ErrorCount       int                     `json:"error_count"`
	CreatedQuestions []uint                  `json:"created_questions"`
	Errors           []ImportValidationError `json:"errors"`
	ProcessingTime   time.Duration           `json:"processing_time"`
}

type ExportRequest struct {
	AssessmentID   *uint      `json:"assessment_id"`
	QuestionBankID *uint      `json:"question_bank_id"`
	Format         string     `json:"format" validate:"oneof=xlsx csv json"`
	IncludeStats   bool       `json:"include_stats"`
	IncludeAnswers bool       `json:"include_answers"`
	Categories     []uint     `json:"categories"`
	QuestionTypes  []string   `json:"question_types"`
	DateFrom       *time.Time `json:"date_from"`
	DateTo         *time.Time `json:"date_to"`
}
