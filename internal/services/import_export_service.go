package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/validator"
	"github.com/xuri/excelize/v2"
)

// ImportExportService handles file import/export operations for questions and assessments
type ImportExportService interface {
	// Import operations
	ImportQuestionsFromFile(ctx context.Context, file multipart.File, filename string, creatorID string) (*ImportResult, error)
	ImportQuestionsFromCSV(ctx context.Context, reader io.Reader, creatorID string) (*ImportResult, error)
	ImportQuestionsFromExcel(ctx context.Context, reader io.Reader, creatorID string) (*ImportResult, error)

	// Export operations
	ExportQuestionsToCSV(ctx context.Context, questionIDs []uint, userID string) ([]byte, error)
	ExportQuestionsToExcel(ctx context.Context, questionIDs []uint, userID string) ([]byte, error)
	ExportAssessmentResults(ctx context.Context, assessmentID uint, userID string) ([]byte, error)

	// Job management
	GetImportJob(ctx context.Context, jobID string) (*models.ImportJob, error)
	ProcessImportJobAsync(ctx context.Context, jobID string) error
}

type importExportService struct {
	repo              repositories.Repository
	logger            *slog.Logger
	validator         *validator.Validator
	assessmentService AssessmentService // Injected dependency
}

// NewImportExportService creates a new ImportExportService with dependencies
func NewImportExportService(repo repositories.Repository, logger *slog.Logger, validator *validator.Validator) ImportExportService {
	return &importExportService{
		repo:      repo,
		logger:    logger,
		validator: validator,
	}
}

// NewImportExportServiceWithDeps creates a new ImportExportService with all dependencies injected
func NewImportExportServiceWithDeps(repo repositories.Repository, logger *slog.Logger, validator *validator.Validator, assessmentService AssessmentService) ImportExportService {
	return &importExportService{
		repo:              repo,
		logger:            logger,
		validator:         validator,
		assessmentService: assessmentService,
	}
}

// ===== IMPORT OPERATIONS =====

// MaxImportRows is the maximum number of rows allowed in a single import (excluding header)
const MaxImportRows = 1000

type ImportResult struct {
	JobID         string                         `json:"job_id"`
	TotalRows     int                            `json:"total_rows"`
	ProcessedRows int                            `json:"processed_rows"`
	SuccessCount  int                            `json:"success_count"`
	ErrorCount    int                            `json:"error_count"`
	Errors        []models.ImportValidationError `json:"errors"`
	QuestionIDs   []uint                         `json:"question_ids,omitempty"` // Return only IDs instead of full objects
	Status        models.ImportJobStatus         `json:"status"`
}

func (s *importExportService) ImportQuestionsFromFile(ctx context.Context, file multipart.File, filename string, creatorID string) (*ImportResult, error) {
	s.logger.Info("Starting file import", "filename", filename, "creator_id", creatorID)

	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".csv":
		return s.ImportQuestionsFromCSV(ctx, file, creatorID)
	case ".xlsx", ".xls":
		return s.ImportQuestionsFromExcel(ctx, file, creatorID)
	default:
		return nil, NewValidationError("file", "unsupported file format", ext)
	}
}

func (s *importExportService) ImportQuestionsFromCSV(ctx context.Context, reader io.Reader, creatorID string) (*ImportResult, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true

	// Read all records
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, NewValidationError("file", "CSV must have header row and at least one data row", len(records))
	}

	// Validate max rows limit
	dataRows := len(records) - 1
	if dataRows > MaxImportRows {
		return nil, NewValidationError("file", fmt.Sprintf("too many rows (%d). Maximum allowed: %d", dataRows, MaxImportRows), dataRows)
	}

	// Parse header
	headers := records[0]
	headerMap := make(map[string]int)
	for i, header := range headers {
		headerMap[strings.ToLower(strings.TrimSpace(header))] = i
	}

	// Validate required columns (correct_answer not required for essay type)
	requiredColumns := []string{"question_type", "question_text"}
	for _, col := range requiredColumns {
		if _, exists := headerMap[col]; !exists {
			return nil, NewValidationError("headers", fmt.Sprintf("missing required column: %s", col), col)
		}
	}

	result := &ImportResult{
		TotalRows: len(records) - 1, // Exclude header
		Status:    models.ImportProcessing,
	}

	var questions []*models.Question
	var errors []models.ImportValidationError

	// Process each data row
	for rowIndex, record := range records[1:] {
		question, rowErrors := s.parseCSVRow(record, headerMap, rowIndex+2, creatorID)
		if len(rowErrors) > 0 {
			errors = append(errors, rowErrors...)
			result.ErrorCount++
		} else if question != nil {
			questions = append(questions, question)
			result.SuccessCount++
		}
		result.ProcessedRows++
	}

	// Save valid questions
	if len(questions) > 0 {
		if err := s.saveImportedQuestions(ctx, questions); err != nil {
			return nil, fmt.Errorf("failed to save questions: %w", err)
		}
	}

	// Extract question IDs for response (avoid returning full objects)
	result.QuestionIDs = extractQuestionIDs(questions)
	result.Errors = errors
	result.Status = models.ImportCompleted

	s.logger.Info("CSV import completed",
		"total_rows", result.TotalRows,
		"success_count", result.SuccessCount,
		"error_count", result.ErrorCount)

	return result, nil
}

func (s *importExportService) ImportQuestionsFromExcel(ctx context.Context, reader io.Reader, creatorID string) (*ImportResult, error) {
	// Read file into memory
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Open Excel file using bytes.NewReader for proper binary handling
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	// Get first sheet
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, NewValidationError("file", "Excel file has no sheets", nil)
	}

	sheetName := sheets[0]
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read Excel rows: %w", err)
	}

	if len(rows) < 2 {
		return nil, NewValidationError("file", "Excel must have header row and at least one data row", len(rows))
	}

	// Validate max rows limit
	dataRows := len(rows) - 1
	if dataRows > MaxImportRows {
		return nil, NewValidationError("file", fmt.Sprintf("too many rows (%d). Maximum allowed: %d", dataRows, MaxImportRows), dataRows)
	}

	// Parse header
	headers := rows[0]
	headerMap := make(map[string]int)
	for i, header := range headers {
		headerMap[strings.ToLower(strings.TrimSpace(header))] = i
	}

	// Validate required columns (same as CSV)
	requiredColumns := []string{"question_type", "question_text"}
	for _, col := range requiredColumns {
		if _, exists := headerMap[col]; !exists {
			return nil, NewValidationError("headers", fmt.Sprintf("missing required column: %s", col), col)
		}
	}

	result := &ImportResult{
		TotalRows: len(rows) - 1,
		Status:    models.ImportProcessing,
	}

	var questions []*models.Question
	var errors []models.ImportValidationError

	// Process each data row
	for rowIndex, row := range rows[1:] {
		question, rowErrors := s.parseExcelRow(row, headerMap, rowIndex+2, creatorID)
		if len(rowErrors) > 0 {
			errors = append(errors, rowErrors...)
			result.ErrorCount++
		} else if question != nil {
			questions = append(questions, question)
			result.SuccessCount++
		}
		result.ProcessedRows++
	}

	// Save valid questions
	if len(questions) > 0 {
		if err := s.saveImportedQuestions(ctx, questions); err != nil {
			return nil, fmt.Errorf("failed to save questions: %w", err)
		}
	}

	// Extract question IDs for response (avoid returning full objects)
	result.QuestionIDs = extractQuestionIDs(questions)
	result.Errors = errors
	result.Status = models.ImportCompleted

	s.logger.Info("Excel import completed",
		"total_rows", result.TotalRows,
		"success_count", result.SuccessCount,
		"error_count", result.ErrorCount)

	return result, nil
}

// ===== EXPORT OPERATIONS =====

func (s *importExportService) ExportQuestionsToCSV(ctx context.Context, questionIDs []uint, userID string) ([]byte, error) {
	questions, err := s.getQuestionsForExport(ctx, questionIDs, userID)
	if err != nil {
		return nil, err
	}

	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Write header
	headers := []string{
		"Question Type", "Question Text", "Option A", "Option B", "Option C", "Option D",
		"Correct Answer", "Points", "Category", "Difficulty", "Tags", "Explanation",
	}
	if err := writer.Write(headers); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data rows
	for _, question := range questions {
		row := s.questionToCSVRow(question)
		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("CSV writer error: %w", err)
	}

	return []byte(buf.String()), nil
}

func (s *importExportService) ExportQuestionsToExcel(ctx context.Context, questionIDs []uint, userID string) ([]byte, error) {
	questions, err := s.getQuestionsForExport(ctx, questionIDs, userID)
	if err != nil {
		return nil, err
	}

	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Questions"

	// Create sheet
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to create Excel sheet: %w", err)
	}
	f.SetActiveSheet(index)

	// Delete default "Sheet1"
	f.DeleteSheet("Sheet1")

	// Write headers
	headers := []string{
		"Question Type", "Question Text", "Option A", "Option B", "Option C", "Option D",
		"Correct Answer", "Points", "Category", "Difficulty", "Tags", "Explanation",
	}

	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue(sheetName, cell, header)
	}

	// Write data
	for rowIndex, question := range questions {
		row := s.questionToCSVRow(question)
		for colIndex, value := range row {
			cell := fmt.Sprintf("%c%d", 'A'+colIndex, rowIndex+2)
			f.SetCellValue(sheetName, cell, value)
		}
	}

	// Save to buffer
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to write Excel file: %w", err)
	}

	return buf.Bytes(), nil
}

func (s *importExportService) ExportAssessmentResults(ctx context.Context, assessmentID uint, userID string) ([]byte, error) {
	// Check permission using injected assessmentService
	var assessmentSvc AssessmentService
	if s.assessmentService != nil {
		assessmentSvc = s.assessmentService
	} else {
		// Fallback for backward compatibility (not recommended)
		assessmentSvc = NewAssessmentService(s.repo, nil, s.logger, s.validator)
	}

	canAccess, err := assessmentSvc.CanAccess(ctx, assessmentID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, assessmentID, "assessment", "export_results", "not owner or insufficient permissions")
	}

	// Get assessment attempts with results
	attempts, _, err := s.repo.Attempt().GetByAssessment(ctx, nil, assessmentID, repositories.AttemptFilters{})
	if err != nil {
		return nil, fmt.Errorf("failed to get assessment attempts: %w", err)
	}

	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Results"

	index, err := f.NewSheet(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to create Excel sheet: %w", err)
	}
	f.SetActiveSheet(index)

	// Write headers
	headers := []string{
		"Student ID", "Student Name", "Attempt", "Status", "Started At", "Submitted At",
		"Total Score", "Percentage", "Grade", "Is Passing", "Time Spent (minutes)",
	}

	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue(sheetName, cell, header)
	}

	// Write attempt data
	for rowIndex, attempt := range attempts {
		row := []interface{}{
			attempt.StudentID,
			attempt.Student.FullName,
			attempt.AttemptNumber,
			string(attempt.Status),
			attempt.StartedAt.Format("2006-01-02 15:04:05"),
		}

		if attempt.CompletedAt != nil {
			row = append(row, attempt.CompletedAt.Format("2006-01-02 15:04:05"))
		} else {
			row = append(row, "")
		}

		row = append(row, attempt.Score)

		row = append(row, attempt.Percentage)

		if attempt.Passed {
			row = append(row, "Pass")
		} else {
			row = append(row, "Fail")
		}

		// Skip IsPassing field as it doesn't exist

		row = append(row, attempt.TimeSpent/60) // Convert seconds to minutes

		for colIndex, value := range row {
			cell := fmt.Sprintf("%c%d", 'A'+colIndex, rowIndex+2)
			f.SetCellValue(sheetName, cell, value)
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to write Excel file: %w", err)
	}

	return buf.Bytes(), nil
}

// ===== JOB MANAGEMENT =====

func (s *importExportService) GetImportJob(ctx context.Context, jobID string) (*models.ImportJob, error) {
	// TODO: Implement job storage and retrieval
	// For now, return a placeholder
	return &models.ImportJob{
		ID:     jobID,
		Status: "completed",
	}, nil
}

func (s *importExportService) ProcessImportJobAsync(ctx context.Context, jobID string) error {
	// TODO: Implement async job processing
	// This would typically involve:
	// 1. Get job from storage
	// 2. Process file in background
	// 3. Update job status and progress
	// 4. Store results
	return nil
}

// ===== HELPER FUNCTIONS =====

func (s *importExportService) parseCSVRow(record []string, headerMap map[string]int, rowNum int, creatorID string) (*models.Question, []models.ImportValidationError) {
	var errors []models.ImportValidationError

	// Helper function to get column value
	getColumn := func(name string) string {
		if index, exists := headerMap[name]; exists && index < len(record) {
			return strings.TrimSpace(record[index])
		}
		return ""
	}

	// ===== VALIDATE QUESTION TYPE =====
	questionTypeStr := getColumn("question_type")
	if questionTypeStr == "" {
		errors = append(errors, models.ImportValidationError{
			Row: rowNum, Column: "question_type", Message: "required field", Value: questionTypeStr, Code: "REQUIRED",
		})
		return nil, errors
	}

	questionType := models.QuestionType(strings.ToLower(questionTypeStr))

	// Validate question type is one of the allowed types
	validTypes := []models.QuestionType{
		models.MultipleChoice, models.TrueFalse, models.Essay,
		models.ShortAnswer, models.FillInBlank, models.Matching, models.Ordering,
	}
	isValidType := false
	for _, vt := range validTypes {
		if questionType == vt {
			isValidType = true
			break
		}
	}
	if !isValidType {
		errors = append(errors, models.ImportValidationError{
			Row:     rowNum,
			Column:  "question_type",
			Message: fmt.Sprintf("invalid question type '%s'. Valid types: multiple_choice, true_false, essay, short_answer, fill_blank, matching, ordering", questionTypeStr),
			Value:   questionTypeStr,
			Code:    "INVALID_TYPE",
		})
		return nil, errors
	}

	// ===== VALIDATE QUESTION TEXT =====
	questionText := getColumn("question_text")
	if questionText == "" {
		errors = append(errors, models.ImportValidationError{
			Row: rowNum, Column: "question_text", Message: "required field", Value: questionText, Code: "REQUIRED",
		})
		return nil, errors
	}

	// Validate text length (max 10000 chars like in model)
	if len(questionText) > 10000 {
		errors = append(errors, models.ImportValidationError{
			Row:     rowNum,
			Column:  "question_text",
			Message: fmt.Sprintf("question text too long (%d chars). Maximum allowed: 10000", len(questionText)),
			Value:   questionText[:50] + "...",
			Code:    "MAX_LENGTH",
		})
		return nil, errors
	}

	// ===== VALIDATE POINTS (1-100) =====
	pointsStr := getColumn("points")
	points := 10 // Default
	if pointsStr != "" {
		p, err := strconv.Atoi(pointsStr)
		if err != nil {
			errors = append(errors, models.ImportValidationError{
				Row:     rowNum,
				Column:  "points",
				Message: fmt.Sprintf("invalid number format '%s'. Must be an integer", pointsStr),
				Value:   pointsStr,
				Code:    "INVALID_FORMAT",
			})
			return nil, errors
		}
		if p < 1 || p > 100 {
			errors = append(errors, models.ImportValidationError{
				Row:     rowNum,
				Column:  "points",
				Message: fmt.Sprintf("points must be between 1 and 100, got %d", p),
				Value:   pointsStr,
				Code:    "OUT_OF_RANGE",
			})
			return nil, errors
		}
		points = p
	}

	// ===== VALIDATE DIFFICULTY =====
	difficultyStr := getColumn("difficulty")
	difficulty := models.DifficultyMedium // Default
	if difficultyStr != "" {
		diffLower := strings.ToLower(difficultyStr)
		switch diffLower {
		case "easy":
			difficulty = models.DifficultyEasy
		case "medium":
			difficulty = models.DifficultyMedium
		case "hard":
			difficulty = models.DifficultyHard
		default:
			errors = append(errors, models.ImportValidationError{
				Row:     rowNum,
				Column:  "difficulty",
				Message: fmt.Sprintf("invalid difficulty '%s'. Valid values: easy, medium, hard", difficultyStr),
				Value:   difficultyStr,
				Code:    "INVALID_VALUE",
			})
			return nil, errors
		}
	}

	// ===== VALIDATE TAGS (max 10, no empty) =====
	tagsStr := getColumn("tags")
	var tags []string
	if tagsStr != "" {
		rawTags := strings.Split(tagsStr, ",")
		for _, tag := range rawTags {
			trimmedTag := strings.TrimSpace(tag)
			if trimmedTag != "" {
				tags = append(tags, trimmedTag)
			}
		}

		// Validate max 10 tags
		if len(tags) > 10 {
			errors = append(errors, models.ImportValidationError{
				Row:     rowNum,
				Column:  "tags",
				Message: fmt.Sprintf("too many tags (%d). Maximum allowed: 10", len(tags)),
				Value:   tagsStr,
				Code:    "MAX_COUNT",
			})
			return nil, errors
		}

		// Validate tag length (max 50 chars each)
		for i, tag := range tags {
			if len(tag) > 50 {
				errors = append(errors, models.ImportValidationError{
					Row:     rowNum,
					Column:  "tags",
					Message: fmt.Sprintf("tag #%d '%s...' too long. Maximum: 50 characters", i+1, tag[:20]),
					Value:   tag,
					Code:    "MAX_LENGTH",
				})
				return nil, errors
			}
		}
	}

	// ===== PARSE AND VALIDATE CONTENT =====
	content, contentErrors := s.parseQuestionContent(questionType, record, headerMap, rowNum)
	if len(contentErrors) > 0 {
		errors = append(errors, contentErrors...)
		return nil, errors
	}

	contentBytes, err := json.Marshal(content)
	if err != nil {
		errors = append(errors, models.ImportValidationError{
			Row: rowNum, Column: "content", Message: "failed to serialize content", Value: "", Code: "INTERNAL_ERROR",
		})
		return nil, errors
	}

	// ===== VALIDATE EXPLANATION (optional, max 5000 chars) =====
	explanation := getColumn("explanation")
	var explanationPtr *string
	if explanation != "" {
		if len(explanation) > 5000 {
			errors = append(errors, models.ImportValidationError{
				Row:     rowNum,
				Column:  "explanation",
				Message: fmt.Sprintf("explanation too long (%d chars). Maximum: 5000", len(explanation)),
				Value:   explanation[:50] + "...",
				Code:    "MAX_LENGTH",
			})
			return nil, errors
		}
		explanationPtr = &explanation
	}

	// ===== BUILD QUESTION =====
	tagsJson, _ := json.Marshal(tags)

	question := &models.Question{
		Type:        questionType,
		Text:        questionText,
		Content:     contentBytes,
		Points:      points,
		Difficulty:  difficulty,
		Tags:        tagsJson,
		Explanation: explanationPtr,
		CreatedBy:   creatorID,
	}

	return question, errors
}

func (s *importExportService) parseExcelRow(record []string, headerMap map[string]int, rowNum int, creatorID string) (*models.Question, []models.ImportValidationError) {
	// Excel parsing is similar to CSV, just different input format
	return s.parseCSVRow(record, headerMap, rowNum, creatorID)
}

func (s *importExportService) parseQuestionContent(questionType models.QuestionType, record []string, headerMap map[string]int, rowNum int) (interface{}, []models.ImportValidationError) {
	var errors []models.ImportValidationError

	getColumn := func(name string) string {
		if index, exists := headerMap[name]; exists && index < len(record) {
			return strings.TrimSpace(record[index])
		}
		return ""
	}

	switch questionType {
	case models.MultipleChoice:
		return s.parseMultipleChoiceContent(record, headerMap, rowNum)

	case models.TrueFalse:
		correctAnswer := strings.ToLower(getColumn("correct_answer"))
		if correctAnswer == "" {
			errors = append(errors, models.ImportValidationError{
				Row:     rowNum,
				Column:  "correct_answer",
				Message: "required for true_false type",
				Value:   correctAnswer,
				Code:    "REQUIRED",
			})
			return nil, errors
		}
		if correctAnswer != "true" && correctAnswer != "false" {
			errors = append(errors, models.ImportValidationError{
				Row:     rowNum,
				Column:  "correct_answer",
				Message: fmt.Sprintf("invalid value '%s'. Must be 'true' or 'false'", correctAnswer),
				Value:   correctAnswer,
				Code:    "INVALID_VALUE",
			})
			return nil, errors
		}
		isTrue := correctAnswer == "true"
		return models.TrueFalseContent{CorrectAnswer: isTrue}, nil

	case models.Essay:
		// Essay questions don't require correct_answer
		return models.EssayContent{}, nil

	case models.ShortAnswer:
		// Short answer requires correct_answer(s)
		correctAnswer := getColumn("correct_answer")
		if correctAnswer == "" {
			errors = append(errors, models.ImportValidationError{
				Row:     rowNum,
				Column:  "correct_answer",
				Message: "required for short_answer type. Use '|' to separate multiple accepted answers",
				Value:   correctAnswer,
				Code:    "REQUIRED",
			})
			return nil, errors
		}
		// Support multiple accepted answers separated by |
		acceptedAnswers := strings.Split(correctAnswer, "|")
		var validAnswers []string
		for _, ans := range acceptedAnswers {
			trimmed := strings.TrimSpace(ans)
			if trimmed != "" {
				validAnswers = append(validAnswers, trimmed)
			}
		}
		if len(validAnswers) == 0 {
			errors = append(errors, models.ImportValidationError{
				Row:     rowNum,
				Column:  "correct_answer",
				Message: "must have at least one non-empty accepted answer",
				Value:   correctAnswer,
				Code:    "INVALID_VALUE",
			})
			return nil, errors
		}
		return models.ShortAnswerContent{
			AcceptedAnswers: validAnswers,
			CaseSensitive:   false,
			ExactMatch:      false,
			MaxLength:       500,
		}, nil

	case models.FillInBlank:
		// Fill in blank format: question_text contains {___} for blanks, correct_answer contains answers separated by |
		// Example: question_text = "The {___} is the capital of {___}", correct_answer = "Paris|France"
		return s.parseFillBlankContent(getColumn("question_text"), getColumn("correct_answer"), rowNum)

	case models.Matching:
		// Matching format: correct_answer contains pairs in format "left1:right1|left2:right2"
		// Example: "Dog:Animal|Cat:Mammal|Eagle:Bird"
		return s.parseMatchingContent(getColumn("correct_answer"), rowNum)

	case models.Ordering:
		// Ordering format: correct_answer contains items in order separated by >
		// Example: "First > Second > Third > Fourth"
		return s.parseOrderingContent(getColumn("correct_answer"), rowNum)

	default:
		errors = append(errors, models.ImportValidationError{
			Row:     rowNum,
			Column:  "question_type",
			Message: fmt.Sprintf("unknown question type '%s'. Supported types: multiple_choice, true_false, essay, short_answer, fill_blank, matching, ordering", string(questionType)),
			Value:   string(questionType),
			Code:    "INVALID_TYPE",
		})
		return nil, errors
	}
}

func (s *importExportService) parseMultipleChoiceContent(record []string, headerMap map[string]int, rowNum int) (interface{}, []models.ImportValidationError) {
	var errors []models.ImportValidationError

	getColumn := func(name string) string {
		if index, exists := headerMap[name]; exists && index < len(record) {
			return strings.TrimSpace(record[index])
		}
		return ""
	}

	// Get options
	var options []models.MCOption
	optionColumns := []string{"option_a", "option_b", "option_c", "option_d"}
	optionLabels := []string{"A", "B", "C", "D"}

	for i, colName := range optionColumns {
		optionText := getColumn(colName)
		if optionText != "" {
			// Validate option text length (max 1000 chars)
			if len(optionText) > 1000 {
				errors = append(errors, models.ImportValidationError{
					Row:     rowNum,
					Column:  colName,
					Message: fmt.Sprintf("option %s text too long (%d chars). Maximum: 1000", optionLabels[i], len(optionText)),
					Value:   optionText[:50] + "...",
					Code:    "MAX_LENGTH",
				})
				return nil, errors
			}
			options = append(options, models.MCOption{
				ID:    fmt.Sprintf("%d", i),
				Text:  optionText,
				Order: i,
			})
		}
	}

	// Validate minimum 2 options (same as Create API)
	if len(options) < 2 {
		errors = append(errors, models.ImportValidationError{
			Row:     rowNum,
			Column:  "options",
			Message: fmt.Sprintf("must have at least 2 options (found %d). Fill in option_a and option_b at minimum", len(options)),
			Value:   fmt.Sprintf("%d options", len(options)),
			Code:    "MIN_COUNT",
		})
		return nil, errors
	}

	// Validate max 10 options
	if len(options) > 10 {
		errors = append(errors, models.ImportValidationError{
			Row:     rowNum,
			Column:  "options",
			Message: fmt.Sprintf("too many options (%d). Maximum: 10", len(options)),
			Value:   fmt.Sprintf("%d options", len(options)),
			Code:    "MAX_COUNT",
		})
		return nil, errors
	}

	// Parse and validate correct answer
	correctAnswerStr := strings.ToUpper(getColumn("correct_answer"))
	if correctAnswerStr == "" {
		errors = append(errors, models.ImportValidationError{
			Row:     rowNum,
			Column:  "correct_answer",
			Message: "required for multiple_choice type. Use A, B, C, D or comma-separated like 'A,C' for multiple correct",
			Value:   correctAnswerStr,
			Code:    "REQUIRED",
		})
		return nil, errors
	}

	var correctAnswers []string
	var invalidAnswers []string

	// Handle multiple correct answers (e.g., "A,C" or "A")
	answerParts := strings.Split(correctAnswerStr, ",")
	for _, part := range answerParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if len(part) == 1 && part >= "A" && part <= "D" {
			index := int(part[0] - 'A')
			if index < len(options) {
				correctAnswers = append(correctAnswers, fmt.Sprintf("%d", index))
			} else {
				invalidAnswers = append(invalidAnswers, fmt.Sprintf("%s (option not provided)", part))
			}
		} else {
			invalidAnswers = append(invalidAnswers, part)
		}
	}

	if len(invalidAnswers) > 0 {
		errors = append(errors, models.ImportValidationError{
			Row:     rowNum,
			Column:  "correct_answer",
			Message: fmt.Sprintf("invalid answer(s): %s. Use only A, B, C, D that correspond to filled options", strings.Join(invalidAnswers, ", ")),
			Value:   correctAnswerStr,
			Code:    "INVALID_VALUE",
		})
		return nil, errors
	}

	if len(correctAnswers) == 0 {
		errors = append(errors, models.ImportValidationError{
			Row:     rowNum,
			Column:  "correct_answer",
			Message: "must specify at least one valid correct answer (A, B, C, or D)",
			Value:   correctAnswerStr,
			Code:    "REQUIRED",
		})
		return nil, errors
	}

	return models.MultipleChoiceContent{
		Options:         options,
		CorrectAnswers:  correctAnswers,
		MultipleCorrect: len(correctAnswers) > 1,
	}, nil
}

func (s *importExportService) saveImportedQuestions(ctx context.Context, questions []*models.Question) error {
	if len(questions) == 0 {
		return nil
	}

	// Use batch insert instead of loop for better performance
	if err := s.repo.Question().CreateBatch(ctx, nil, questions); err != nil {
		return fmt.Errorf("failed to batch create questions: %w", err)
	}
	return nil
}

func (s *importExportService) getQuestionsForExport(ctx context.Context, questionIDs []uint, userID string) ([]*models.Question, error) {
	if len(questionIDs) == 0 {
		return nil, nil
	}

	// Use batch query instead of N+1 individual queries
	allQuestions, err := s.repo.Question().GetByIDs(ctx, nil, questionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get questions: %w", err)
	}

	// Get user role to determine access level
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user role: %w", err)
	}

	// Admin and Teacher can access all questions
	if userRole == models.RoleAdmin || userRole == models.RoleTeacher {
		return allQuestions, nil
	}

	// For other users, filter to only their own questions
	var accessibleQuestions []*models.Question
	for _, question := range allQuestions {
		if question.CreatedBy == userID {
			accessibleQuestions = append(accessibleQuestions, question)
		}
	}

	return accessibleQuestions, nil
}

// getUserRole retrieves the role of a user
func (s *importExportService) getUserRole(ctx context.Context, userID string) (models.Role, error) {
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return models.RoleStudent, nil // Default role
	}
	return user.Role, nil
}

// extractQuestionIDs extracts IDs from question slice to reduce response size
func extractQuestionIDs(questions []*models.Question) []uint {
	ids := make([]uint, len(questions))
	for i, q := range questions {
		ids[i] = q.ID
	}
	return ids
}

func (s *importExportService) questionToCSVRow(question *models.Question) []string {
	row := make([]string, 12) // 12 columns as defined in headers

	row[0] = string(question.Type)
	row[1] = question.Text

	// Parse content for options and correct answer
	switch question.Type {
	case models.MultipleChoice:
		var content models.MultipleChoiceContent
		if err := json.Unmarshal(question.Content, &content); err == nil {
			// Fill options
			for i, option := range content.Options {
				if i < 4 { // A, B, C, D
					row[2+i] = option.Text
				}
			}

			// Fill correct answer
			var correctLetters []string
			for _, optionID := range content.CorrectAnswers {
				// Convert option ID (string) to index
				if idx, err := strconv.Atoi(optionID); err == nil && idx < 4 {
					correctLetters = append(correctLetters, string('A'+rune(idx)))
				}
			}
			row[6] = strings.Join(correctLetters, ",")
		}

	case models.TrueFalse:
		var content models.TrueFalseContent
		if err := json.Unmarshal(question.Content, &content); err == nil {
			if content.CorrectAnswer {
				row[6] = "True"
			} else {
				row[6] = "False"
			}
		}

	case models.ShortAnswer:
		var content models.ShortAnswerContent
		if err := json.Unmarshal(question.Content, &content); err == nil {
			// Join multiple accepted answers with |
			row[6] = strings.Join(content.AcceptedAnswers, "|")
		}

	case models.Essay:
		// Essay doesn't have correct answer
		row[6] = ""

	case models.FillInBlank:
		var content models.FillBlankContent
		if err := json.Unmarshal(question.Content, &content); err == nil {
			// Export answers in order of blanks
			var answers []string
			for i := 1; ; i++ {
				blankID := fmt.Sprintf("blank%d", i)
				if blank, ok := content.Blanks[blankID]; ok {
					answers = append(answers, strings.Join(blank.AcceptedAnswers, "|||"))
				} else {
					break
				}
			}
			row[6] = strings.Join(answers, "|")
		}

	case models.Matching:
		var content models.MatchingContent
		if err := json.Unmarshal(question.Content, &content); err == nil {
			// Build a map for quick lookup
			leftMap := make(map[string]string)
			rightMap := make(map[string]string)
			for _, item := range content.LeftItems {
				leftMap[item.ID] = item.Text
			}
			for _, item := range content.RightItems {
				rightMap[item.ID] = item.Text
			}
			// Export pairs in format "left:right|left:right"
			var pairs []string
			for _, pair := range content.CorrectPairs {
				leftText := leftMap[pair.LeftID]
				rightText := rightMap[pair.RightID]
				pairs = append(pairs, leftText+":"+rightText)
			}
			row[6] = strings.Join(pairs, "|")
		}

	case models.Ordering:
		var content models.OrderingContent
		if err := json.Unmarshal(question.Content, &content); err == nil {
			// Build a map for quick lookup
			itemMap := make(map[string]string)
			for _, item := range content.Items {
				itemMap[item.ID] = item.Text
			}
			// Export items in correct order
			var orderedItems []string
			for _, itemID := range content.CorrectOrder {
				orderedItems = append(orderedItems, itemMap[itemID])
			}
			row[6] = strings.Join(orderedItems, " > ")
		}
	}

	row[7] = strconv.Itoa(question.Points)

	if question.Category != nil {
		row[8] = question.Category.Name
	}

	row[9] = string(question.Difficulty)

	// Handle tags
	var tags []string
	if err := json.Unmarshal(question.Tags, &tags); err == nil {
		row[10] = strings.Join(tags, ",")
	} else {
		row[10] = ""
	}

	if question.Explanation != nil {
		row[11] = *question.Explanation
	}

	return row
}

// parseFillBlankContent parses fill-in-the-blank question content
// Format: question_text contains {___} for blanks, correct_answer contains answers separated by |
// Example: question_text = "The {___} is the capital of {___}", correct_answer = "Paris|France"
func (s *importExportService) parseFillBlankContent(questionText, correctAnswer string, rowNum int) (interface{}, []models.ImportValidationError) {
	var errors []models.ImportValidationError

	// Count blanks in template (look for {___} or similar patterns)
	blankPattern := regexp.MustCompile(`\{___\}|\{[^}]+\}`)
	blanks := blankPattern.FindAllStringIndex(questionText, -1)
	blankCount := len(blanks)

	if blankCount == 0 {
		errors = append(errors, models.ImportValidationError{
			Row:     rowNum,
			Column:  "question_text",
			Message: "fill_blank questions must contain at least one blank marker {___} or {blank1}",
			Value:   questionText,
			Code:    "INVALID_FORMAT",
		})
		return nil, errors
	}

	// Parse answers separated by |
	if correctAnswer == "" {
		errors = append(errors, models.ImportValidationError{
			Row:     rowNum,
			Column:  "correct_answer",
			Message: "required for fill_blank type. Use '|' to separate answers for each blank",
			Value:   correctAnswer,
			Code:    "REQUIRED",
		})
		return nil, errors
	}

	answerParts := strings.Split(correctAnswer, "|")
	if len(answerParts) != blankCount {
		errors = append(errors, models.ImportValidationError{
			Row:     rowNum,
			Column:  "correct_answer",
			Message: fmt.Sprintf("number of answers (%d) must match number of blanks (%d) in question text", len(answerParts), blankCount),
			Value:   correctAnswer,
			Code:    "INVALID_FORMAT",
		})
		return nil, errors
	}

	// Build template and blanks map
	blanksMap := make(map[string]models.BlankDef)
	for i, ans := range answerParts {
		blankID := fmt.Sprintf("blank%d", i+1)
		// Each answer can have multiple accepted answers separated by |||
		acceptedAnswers := strings.Split(strings.TrimSpace(ans), "|||")
		for j := range acceptedAnswers {
			acceptedAnswers[j] = strings.TrimSpace(acceptedAnswers[j])
		}
		blanksMap[blankID] = models.BlankDef{
			AcceptedAnswers: acceptedAnswers,
			Points:          1,
		}
	}

	// Replace {___} with {blank1}, {blank2}, etc. in template
	template := questionText
	for i := 0; i < blankCount; i++ {
		template = strings.Replace(template, "{___}", fmt.Sprintf("{blank%d}", i+1), 1)
	}

	return models.FillBlankContent{
		Template:      template,
		Blanks:        blanksMap,
		CaseSensitive: false,
		TrimSpaces:    true,
	}, nil
}

// parseMatchingContent parses matching question content
// Format: correct_answer contains pairs in format "left1:right1|left2:right2"
// Example: "Dog:Animal|Cat:Mammal|Eagle:Bird"
func (s *importExportService) parseMatchingContent(correctAnswer string, rowNum int) (interface{}, []models.ImportValidationError) {
	var errors []models.ImportValidationError

	if correctAnswer == "" {
		errors = append(errors, models.ImportValidationError{
			Row:     rowNum,
			Column:  "correct_answer",
			Message: "required for matching type. Format: left1:right1|left2:right2",
			Value:   correctAnswer,
			Code:    "REQUIRED",
		})
		return nil, errors
	}

	pairs := strings.Split(correctAnswer, "|")
	if len(pairs) < 2 {
		errors = append(errors, models.ImportValidationError{
			Row:     rowNum,
			Column:  "correct_answer",
			Message: "matching questions require at least 2 pairs. Format: left1:right1|left2:right2",
			Value:   correctAnswer,
			Code:    "MIN_COUNT",
		})
		return nil, errors
	}

	var leftItems []models.MatchItem
	var rightItems []models.MatchItem
	var correctPairs []models.MatchPair

	for i, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) != 2 {
			errors = append(errors, models.ImportValidationError{
				Row:     rowNum,
				Column:  "correct_answer",
				Message: fmt.Sprintf("invalid pair format '%s'. Use 'left:right' format", pair),
				Value:   pair,
				Code:    "INVALID_FORMAT",
			})
			return nil, errors
		}

		leftID := fmt.Sprintf("L%d", i+1)
		rightID := fmt.Sprintf("R%d", i+1)

		leftItems = append(leftItems, models.MatchItem{
			ID:   leftID,
			Text: strings.TrimSpace(parts[0]),
		})
		rightItems = append(rightItems, models.MatchItem{
			ID:   rightID,
			Text: strings.TrimSpace(parts[1]),
		})
		correctPairs = append(correctPairs, models.MatchPair{
			LeftID:  leftID,
			RightID: rightID,
		})
	}

	return models.MatchingContent{
		LeftItems:      leftItems,
		RightItems:     rightItems,
		CorrectPairs:   correctPairs,
		RandomizeLeft:  true,
		RandomizeRight: true,
		PartialCredit:  true,
	}, nil
}

// parseOrderingContent parses ordering question content
// Format: correct_answer contains items in order separated by >
// Example: "First > Second > Third > Fourth"
func (s *importExportService) parseOrderingContent(correctAnswer string, rowNum int) (interface{}, []models.ImportValidationError) {
	var errors []models.ImportValidationError

	if correctAnswer == "" {
		errors = append(errors, models.ImportValidationError{
			Row:     rowNum,
			Column:  "correct_answer",
			Message: "required for ordering type. Format: item1 > item2 > item3",
			Value:   correctAnswer,
			Code:    "REQUIRED",
		})
		return nil, errors
	}

	items := strings.Split(correctAnswer, ">")
	if len(items) < 2 {
		errors = append(errors, models.ImportValidationError{
			Row:     rowNum,
			Column:  "correct_answer",
			Message: "ordering questions require at least 2 items. Format: item1 > item2 > item3",
			Value:   correctAnswer,
			Code:    "MIN_COUNT",
		})
		return nil, errors
	}

	var orderItems []models.OrderItem
	var correctOrder []string

	for i, item := range items {
		itemID := fmt.Sprintf("item%d", i+1)
		orderItems = append(orderItems, models.OrderItem{
			ID:   itemID,
			Text: strings.TrimSpace(item),
		})
		correctOrder = append(correctOrder, itemID)
	}

	return models.OrderingContent{
		Items:         orderItems,
		CorrectOrder:  correctOrder,
		RandomizeInit: true,
		PartialCredit: true,
	}, nil
}
