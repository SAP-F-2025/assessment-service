package services

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"path/filepath"
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
	repo      repositories.Repository
	logger    *slog.Logger
	validator *validator.Validator
}

func NewImportExportService(repo repositories.Repository, logger *slog.Logger, validator *validator.Validator) ImportExportService {
	return &importExportService{
		repo:      repo,
		logger:    logger,
		validator: validator,
	}
}

// ===== IMPORT OPERATIONS =====

type ImportResult struct {
	JobID         string                         `json:"job_id"`
	TotalRows     int                            `json:"total_rows"`
	ProcessedRows int                            `json:"processed_rows"`
	SuccessCount  int                            `json:"success_count"`
	ErrorCount    int                            `json:"error_count"`
	Errors        []models.ImportValidationError `json:"errors"`
	Questions     []*models.Question             `json:"questions,omitempty"`
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

	// Parse header
	headers := records[0]
	headerMap := make(map[string]int)
	for i, header := range headers {
		headerMap[strings.ToLower(strings.TrimSpace(header))] = i
	}

	// Validate required columns
	requiredColumns := []string{"question_type", "question_text", "correct_answer"}
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

	result.Questions = questions
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

	// Open Excel file
	f, err := excelize.OpenReader(strings.NewReader(string(data)))
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

	// Parse header
	headers := rows[0]
	headerMap := make(map[string]int)
	for i, header := range headers {
		headerMap[strings.ToLower(strings.TrimSpace(header))] = i
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

	result.Questions = questions
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
	sheetName := "Questions"

	// Create sheet
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to create Excel sheet: %w", err)
	}
	f.SetActiveSheet(index)

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
	// Check permission
	assessmentService := NewAssessmentService(s.repo, nil, s.logger, s.validator)
	canAccess, err := assessmentService.CanAccess(ctx, assessmentID, userID)
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

	// Parse question type
	questionTypeStr := getColumn("question_type")
	if questionTypeStr == "" {
		errors = append(errors, models.ImportValidationError{
			Row: rowNum, Column: "question_type", Message: "required field", Value: questionTypeStr,
		})
		return nil, errors
	}

	questionType := models.QuestionType(strings.ToLower(questionTypeStr))

	// Parse question text
	questionText := getColumn("question_text")
	if questionText == "" {
		errors = append(errors, models.ImportValidationError{
			Row: rowNum, Column: "question_text", Message: "required field", Value: questionText,
		})
		return nil, errors
	}

	// Parse points
	pointsStr := getColumn("points")
	points := 10 // Default
	if pointsStr != "" {
		if p, err := strconv.Atoi(pointsStr); err == nil && p > 0 {
			points = p
		}
	}

	// Parse difficulty
	difficultyStr := getColumn("difficulty")
	difficulty := models.DifficultyMedium // Default
	if difficultyStr != "" {
		switch strings.ToLower(difficultyStr) {
		case "easy":
			difficulty = models.DifficultyEasy
		case "medium":
			difficulty = models.DifficultyMedium
		case "hard":
			difficulty = models.DifficultyHard
		}
	}

	// Parse content based on question type
	content, contentErrors := s.parseQuestionContent(questionType, record, headerMap, rowNum)
	if len(contentErrors) > 0 {
		errors = append(errors, contentErrors...)
		return nil, errors
	}

	contentBytes, err := json.Marshal(content)
	if err != nil {
		errors = append(errors, models.ImportValidationError{
			Row: rowNum, Column: "content", Message: "failed to serialize content", Value: "",
		})
		return nil, errors
	}

	// Parse tags
	tagsStr := getColumn("tags")
	var tags []string
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}

	// Parse explanation
	explanation := getColumn("explanation")
	var explanationPtr *string
	if explanation != "" {
		explanationPtr = &explanation
	}

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
		if correctAnswer != "true" && correctAnswer != "false" {
			errors = append(errors, models.ImportValidationError{
				Row: rowNum, Column: "correct_answer", Message: "must be 'true' or 'false'", Value: correctAnswer,
			})
			return nil, errors
		}
		isTrue := correctAnswer == "true"
		return models.TrueFalseContent{CorrectAnswer: isTrue}, nil
	case models.Essay:
		return models.EssayContent{}, nil
	default:
		errors = append(errors, models.ImportValidationError{
			Row: rowNum, Column: "question_type", Message: "unsupported question type", Value: string(questionType),
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

	for i, colName := range optionColumns {
		optionText := getColumn(colName)
		if optionText != "" {
			options = append(options, models.MCOption{
				ID:    fmt.Sprintf("%d", i),
				Text:  optionText,
				Order: i,
			})
		}
	}

	if len(options) < 2 {
		errors = append(errors, models.ImportValidationError{
			Row: rowNum, Column: "options", Message: "must have at least 2 options", Value: "",
		})
		return nil, errors
	}

	// Parse correct answer
	correctAnswerStr := strings.ToUpper(getColumn("correct_answer"))
	var correctAnswers []string

	if correctAnswerStr != "" {
		// Handle multiple correct answers (e.g., "A,C" or "A")
		answerParts := strings.Split(correctAnswerStr, ",")
		for _, part := range answerParts {
			part = strings.TrimSpace(part)
			if len(part) == 1 && part >= "A" && part <= "D" {
				index := int(part[0] - 'A')
				if index < len(options) {
					correctAnswers = append(correctAnswers, fmt.Sprintf("%d", index))
				}
			}
		}
	}

	if len(correctAnswers) == 0 {
		errors = append(errors, models.ImportValidationError{
			Row: rowNum, Column: "correct_answer", Message: "must specify at least one correct answer (A, B, C, or D)", Value: correctAnswerStr,
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
	// Begin transaction
	txRepo, err := s.repo.(repositories.TransactionRepository).Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			txRepo.(repositories.TransactionRepository).Rollback(ctx)
		}
	}()

	// Save questions
	for _, question := range questions {
		if err := txRepo.Question().Create(ctx, nil, question); err != nil {
			return fmt.Errorf("failed to create question: %w", err)
		}
	}

	// Commit transaction
	if err := txRepo.(repositories.TransactionRepository).Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *importExportService) getQuestionsForExport(ctx context.Context, questionIDs []uint, userID string) ([]*models.Question, error) {
	var questions []*models.Question

	for _, questionID := range questionIDs {
		question, err := s.repo.Question().GetByIDWithDetails(ctx, nil, questionID)
		if err != nil {
			if repositories.IsNotFoundError(err) {
				continue // Skip missing questions
			}
			return nil, fmt.Errorf("failed to get question %d: %w", questionID, err)
		}

		// Check access permission
		questionService := NewQuestionService(s.repo, nil, s.logger, s.validator)
		canAccess, err := questionService.CanAccess(ctx, questionID, userID)
		if err != nil {
			return nil, err
		}
		if !canAccess {
			continue // Skip inaccessible questions
		}

		questions = append(questions, question)
	}

	return questions, nil
}

func (s *importExportService) questionToCSVRow(question *models.Question) []string {
	row := make([]string, 12) // 12 columns as defined in headers

	row[0] = string(question.Type)
	row[1] = question.Text

	// Parse content for options and correct answer
	if question.Type == models.MultipleChoice {
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
	} else if question.Type == models.TrueFalse {
		var content models.TrueFalseContent
		if err := json.Unmarshal(question.Content, &content); err == nil {
			if content.CorrectAnswer {
				row[6] = "True"
			} else {
				row[6] = "False"
			}
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
