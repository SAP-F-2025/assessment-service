package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/SAP-F-2025/assessment-service/internal/services"
	"github.com/SAP-F-2025/assessment-service/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

type ImportExportHandler struct {
	service services.ImportExportService
	logger  utils.Logger
}

func NewImportExportHandler(service services.ImportExportService, logger utils.Logger) *ImportExportHandler {
	return &ImportExportHandler{
		service: service,
		logger:  logger,
	}
}

// ImportQuestions godoc
// @Summary Import questions from file
// @Description Import questions from Excel (.xlsx) or CSV (.csv) file
// @Tags Questions
// @Accept multipart/form-data
// @Produce json
// @Param file formance file true "Excel or CSV file containing questions"
// @Success 200 {object} services.ImportResult "Import result with success count and errors"
// @Failure 400 {object} ErrorResponse "Bad request - invalid file or format"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /questions/import [post]
func (h *ImportExportHandler) ImportQuestions(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"message": "User ID not found in context",
		})
		return
	}

	// Get file from form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "No file uploaded or invalid file field. Use 'file' as the form field name.",
		})
		return
	}
	defer file.Close()

	// Validate file extension
	filename := header.Filename
	ext := strings.ToLower(filename[strings.LastIndex(filename, "."):])
	if ext != ".xlsx" && ext != ".xls" && ext != ".csv" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Unsupported file format. Only .xlsx, .xls, and .csv files are supported.",
			"details": gin.H{
				"filename":          filename,
				"extension":         ext,
				"supported_formats": []string{".xlsx", ".xls", ".csv"},
			},
		})
		return
	}

	// Validate file size (max 10MB)
	maxSize := int64(10 * 1024 * 1024) // 10MB
	if header.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "File size exceeds maximum allowed size (10MB)",
			"details": gin.H{
				"file_size":    header.Size,
				"max_size":     maxSize,
				"file_size_mb": float64(header.Size) / (1024 * 1024),
			},
		})
		return
	}

	h.logger.Info("Importing questions from file",
		"filename", filename,
		"size", header.Size,
		"user_id", userID,
	)

	// Process import
	result, err := h.service.ImportQuestionsFromFile(c.Request.Context(), file, filename, userID)
	if err != nil {
		h.logger.Error("Failed to import questions",
			"error", err,
			"filename", filename,
			"user_id", userID,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Import Failed",
			"message": err.Error(),
		})
		return
	}

	// Return result
	c.JSON(http.StatusOK, gin.H{
		"message": "Import completed",
		"data":    result,
	})
}

// ExportQuestions godoc
// @Summary Export questions to file
// @Description Export selected questions to Excel (.xlsx) or CSV (.csv) format
// @Tags Questions
// @Produce application/octet-stream
// @Param question_ids query string true "Comma-separated list of question IDs to export"
// @Param format query string false "Export format: xlsx or csv" default(xlsx)
// @Success 200 {file} file "Exported file"
// @Failure 400 {object} ErrorResponse "Bad request - invalid parameters"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /questions/export [get]
func (h *ImportExportHandler) ExportQuestions(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"message": "User ID not found in context",
		})
		return
	}

	// Parse question IDs
	questionIDsStr := c.Query("question_ids")
	if questionIDsStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "question_ids parameter is required",
		})
		return
	}

	var questionIDs []uint
	for _, idStr := range strings.Split(questionIDsStr, ",") {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Bad Request",
				"message": "Invalid question ID: " + idStr,
			})
			return
		}
		questionIDs = append(questionIDs, uint(id))
	}

	if len(questionIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "At least one valid question ID is required",
		})
		return
	}

	// Get format (default: xlsx)
	format := strings.ToLower(c.DefaultQuery("format", "xlsx"))
	if format != "xlsx" && format != "csv" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Unsupported export format. Use 'xlsx' or 'csv'.",
		})
		return
	}

	h.logger.Info("Exporting questions",
		"question_ids", questionIDs,
		"format", format,
		"user_id", userID,
	)

	// Export questions
	var data []byte
	var err error
	var contentType string
	var filename string

	if format == "xlsx" {
		data, err = h.service.ExportQuestionsToExcel(c.Request.Context(), questionIDs, userID)
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		filename = "questions_export.xlsx"
	} else {
		data, err = h.service.ExportQuestionsToCSV(c.Request.Context(), questionIDs, userID)
		contentType = "text/csv"
		filename = "questions_export.csv"
	}

	if err != nil {
		h.logger.Error("Failed to export questions",
			"error", err,
			"format", format,
			"user_id", userID,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Export Failed",
			"message": err.Error(),
		})
		return
	}

	// Set response headers and send file
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.Itoa(len(data)))
	c.Data(http.StatusOK, contentType, data)
}

// DownloadTemplate godoc
// @Summary Download import template
// @Description Download an Excel template file with headers and example data for importing questions
// @Tags Questions
// @Produce application/octet-stream
// @Success 200 {file} file "Template Excel file"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /questions/template [get]
func (h *ImportExportHandler) DownloadTemplate(c *gin.Context) {
	h.logger.Info("Generating import template")

	// Create template Excel file
	f := excelize.NewFile()
	sheetName := "Questions"

	// Create sheet
	index, err := f.NewSheet(sheetName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Template Generation Failed",
			"message": err.Error(),
		})
		return
	}
	f.SetActiveSheet(index)

	// Delete default sheet
	f.DeleteSheet("Sheet1")

	// Define headers
	headers := []string{
		"question_type",
		"question_text",
		"option_a",
		"option_b",
		"option_c",
		"option_d",
		"correct_answer",
		"points",
		"difficulty",
		"tags",
		"explanation",
	}

	// Write headers with styling
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Size:  11,
			Color: "#FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#4472C4"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#000000", Style: 1},
			{Type: "right", Color: "#000000", Style: 1},
			{Type: "top", Color: "#000000", Style: 1},
			{Type: "bottom", Color: "#000000", Style: 1},
		},
	})

	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		f.SetCellValue(sheetName, cell, header)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// Add example data rows
	examples := [][]interface{}{
		{"multiple_choice", "What is 2 + 2?", "3", "4", "5", "6", "B", 10, "easy", "math,arithmetic", "Basic addition"},
		{"multiple_choice", "Which are prime numbers?", "2", "4", "5", "8", "A,C", 15, "medium", "math,numbers", "2 and 5 are prime"},
		{"true_false", "The Earth is flat.", "", "", "", "", "False", 5, "easy", "science,geography", "The Earth is approximately spherical"},
		{"essay", "Explain the water cycle.", "", "", "", "", "", 20, "hard", "science,geography", ""},
		{"short_answer", "What is the capital of France?", "", "", "", "", "Paris|paris|PARIS", 10, "easy", "geography", "Paris is the capital city"},
		{"fill_blank", "The {___} is the capital of {___}.", "", "", "", "", "Paris|France", 15, "medium", "geography", "Fill in the blanks"},
		{"matching", "Match animals to their categories", "", "", "", "", "Dog:Mammal|Cat:Mammal|Eagle:Bird|Shark:Fish", 20, "medium", "biology", "Match each animal"},
		{"ordering", "Put the steps in order", "", "", "", "", "First > Second > Third > Fourth", 15, "easy", "process", "Correct sequence"},
	}

	for rowIdx, row := range examples {
		for colIdx, value := range row {
			cell := string(rune('A'+colIdx)) + strconv.Itoa(rowIdx+2)
			f.SetCellValue(sheetName, cell, value)
		}
	}

	// Set column widths
	columnWidths := map[string]float64{
		"A": 18, // question_type
		"B": 40, // question_text
		"C": 15, // option_a
		"D": 15, // option_b
		"E": 15, // option_c
		"F": 15, // option_d
		"G": 40, // correct_answer (wider for complex types)
		"H": 10, // points
		"I": 12, // difficulty
		"J": 20, // tags
		"K": 30, // explanation
	}
	for col, width := range columnWidths {
		f.SetColWidth(sheetName, col, col, width)
	}

	// Create Instructions sheet
	instructionsSheet := "Instructions"
	f.NewSheet(instructionsSheet)

	instructions := [][]string{
		{"Import Template Instructions"},
		{""},
		{"Column", "Description", "Required", "Example Values"},
		{"question_type", "Type of question", "Yes", "multiple_choice, true_false, essay, short_answer, fill_blank, matching, ordering"},
		{"question_text", "The question text", "Yes", "What is the capital of France?"},
		{"option_a", "First option (for multiple choice only)", "Depends", "Paris"},
		{"option_b", "Second option (for multiple choice only)", "Depends", "London"},
		{"option_c", "Third option (optional)", "No", "Berlin"},
		{"option_d", "Fourth option (optional)", "No", "Madrid"},
		{"correct_answer", "Correct answer(s) - format depends on type", "Yes*", "See format guide below"},
		{"points", "Points for this question", "No", "10 (default)"},
		{"difficulty", "Difficulty level", "No", "easy, medium (default), hard"},
		{"tags", "Comma-separated tags", "No", "math,algebra,equations"},
		{"explanation", "Explanation for the answer", "No", "Paris is the capital of France"},
		{""},
		{"=== CORRECT ANSWER FORMAT GUIDE ==="},
		{""},
		{"QUESTION TYPE", "FORMAT", "EXAMPLE"},
		{"multiple_choice", "Single letter or comma-separated letters", "B or A,C,D"},
		{"true_false", "True or False", "True or False"},
		{"essay", "Leave empty", ""},
		{"short_answer", "Accepted answers separated by |", "Paris|paris|PARIS"},
		{"fill_blank", "Answers for each blank separated by |", "Paris|France (for 2 blanks)"},
		{"matching", "Pairs in format left:right separated by |", "Dog:Mammal|Cat:Mammal|Eagle:Bird"},
		{"ordering", "Items in correct order separated by >", "First > Second > Third > Fourth"},
		{""},
		{"=== SPECIAL NOTES ==="},
		{""},
		{"fill_blank:", "Use {___} in question_text to mark blanks. Number of answers must match number of blanks."},
		{"fill_blank:", "For multiple accepted answers per blank, use ||| (triple pipe): Paris|||paris|France"},
		{"matching:", "Each pair format is 'left:right'. Minimum 2 pairs required."},
		{"ordering:", "Items will be randomized during test. Minimum 2 items required."},
	}

	for rowIdx, row := range instructions {
		for colIdx, value := range row {
			cell := string(rune('A'+colIdx)) + strconv.Itoa(rowIdx+1)
			f.SetCellValue(instructionsSheet, cell, value)
		}
	}

	// Style the title
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
			Size: 14,
		},
	})
	f.SetCellStyle(instructionsSheet, "A1", "A1", titleStyle)

	// Set column widths for instructions
	f.SetColWidth(instructionsSheet, "A", "A", 20)
	f.SetColWidth(instructionsSheet, "B", "B", 40)
	f.SetColWidth(instructionsSheet, "C", "C", 12)
	f.SetColWidth(instructionsSheet, "D", "D", 40)

	// Write to buffer
	buf, err := f.WriteToBuffer()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Template Generation Failed",
			"message": err.Error(),
		})
		return
	}

	// Send file
	filename := "questions_import_template.xlsx"
	contentType := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.Itoa(buf.Len()))
	c.Data(http.StatusOK, contentType, buf.Bytes())
}

// GetImportJobStatus godoc
// @Summary Get import job status
// @Description Get the status and progress of an async import job
// @Tags Import Jobs
// @Produce json
// @Param id path string true "Import job ID"
// @Success 200 {object} models.ImportJob "Import job details"
// @Failure 404 {object} ErrorResponse "Job not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /import-jobs/{id} [get]
func (h *ImportExportHandler) GetImportJobStatus(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Job ID is required",
		})
		return
	}

	job, err := h.service.GetImportJob(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get import job",
			"message": err.Error(),
		})
		return
	}

	if job == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Not Found",
			"message": "Import job not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": job,
	})
}

// ExportAssessmentResults godoc
// @Summary Export assessment results
// @Description Export all attempt results for an assessment to Excel
// @Tags Assessments
// @Produce application/octet-stream
// @Param id path int true "Assessment ID"
// @Success 200 {file} file "Results Excel file"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /assessments/{id}/results/export [get]
func (h *ImportExportHandler) ExportAssessmentResults(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"message": "User ID not found in context",
		})
		return
	}

	// Parse assessment ID
	assessmentIDStr := c.Param("id")
	assessmentID, err := strconv.ParseUint(assessmentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid assessment ID",
		})
		return
	}

	h.logger.Info("Exporting assessment results",
		"assessment_id", assessmentID,
		"user_id", userID,
	)

	// Export results
	data, err := h.service.ExportAssessmentResults(c.Request.Context(), uint(assessmentID), userID)
	if err != nil {
		h.logger.Error("Failed to export assessment results",
			"error", err,
			"assessment_id", assessmentID,
			"user_id", userID,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Export Failed",
			"message": err.Error(),
		})
		return
	}

	// Send file
	filename := "assessment_" + assessmentIDStr + "_results.xlsx"
	contentType := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.Itoa(len(data)))
	c.Data(http.StatusOK, contentType, data)
}
