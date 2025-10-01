package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/services"
	"github.com/SAP-F-2025/assessment-service/internal/utils"
	"github.com/SAP-F-2025/assessment-service/internal/validator"
	"github.com/gin-gonic/gin"
)

type QuestionHandler struct {
	BaseHandler
	questionService services.QuestionService
	validator       *validator.Validator
}

func NewQuestionHandler(
	questionService services.QuestionService,
	validator *validator.Validator,
	logger utils.Logger,
) *QuestionHandler {
	return &QuestionHandler{
		BaseHandler:     NewBaseHandler(logger),
		questionService: questionService,
		validator:       validator,
	}
}

// CreateQuestion creates a new question
// @Summary Create question
// @Description Creates a new question with the provided details
// @Tags questions
// @Accept json
// @Produce json
// @Param question body services.CreateQuestionRequest true "Question data"
// @Success 201 {object} SuccessResponse{data=services.QuestionResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /questions [post]
func (h *QuestionHandler) CreateQuestion(c *gin.Context) {
	h.LogRequest(c, "Creating question")

	var req services.CreateQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid request payload",
			Details: err.Error(),
		})
		return
	}

	// convert content to json
	contentData, err := json.Marshal(req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid content format",
			Details: err.Error(),
		})
		return
	}

	if err := h.validator.GetQuestionValidator().ValidateQuestion(&models.Question{
		Type:        req.Type,
		Text:        req.Text,
		Content:     contentData,
		Difficulty:  req.Difficulty,
		CategoryID:  req.CategoryID,
		Points:      req.Points,
		TimeLimit:   req.TimeLimit,
		Explanation: req.Explanation,
	}); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Validation failed",
			Details: err.Error(),
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	question, err := h.questionService.Create(c.Request.Context(), &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, question)
}

// GetQuestion retrieves a question by ID
// @Summary Get question
// @Description Retrieves a question by its ID
// @Tags questions
// @Accept json
// @Produce json
// @Param id path uint true "Question ID"
// @Success 200 {object} SuccessResponse{data=services.QuestionResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /questions/{id} [get]
func (h *QuestionHandler) GetQuestion(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Getting question", "question_id", id)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	question, err := h.questionService.GetByID(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, question)
}

// GetQuestionWithDetails retrieves a question with full details
// @Summary Get question with details
// @Description Retrieves a question with full details
// @Tags questions
// @Accept json
// @Produce json
// @Param id path uint true "Question ID"
// @Success 200 {object} SuccessResponse{data=services.QuestionResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /questions/{id}/details [get]
func (h *QuestionHandler) GetQuestionWithDetails(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Getting question with details", "question_id", id)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	question, err := h.questionService.GetByIDWithDetails(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, question)
}

// UpdateQuestion updates an existing question
// @Summary Update question
// @Description Updates an existing question with the provided details
// @Tags questions
// @Accept json
// @Produce json
// @Param id path uint true "Question ID"
// @Param question body services.UpdateQuestionRequest true "Question update data"
// @Success 200 {object} SuccessResponse{data=services.QuestionResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /questions/{id} [put]
func (h *QuestionHandler) UpdateQuestion(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Updating question", "question_id", id)

	var req services.UpdateQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid request payload",
			Details: err.Error(),
		})
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Validation failed",
			Details: err.Error(),
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	question, err := h.questionService.Update(c.Request.Context(), id, &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, question)
}

// DeleteQuestion deletes a question
// @Summary Delete question
// @Description Deletes a question by ID
// @Tags questions
// @Accept json
// @Produce json
// @Param id path uint true "Question ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /questions/{id} [delete]
func (h *QuestionHandler) DeleteQuestion(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Deleting question", "question_id", id)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	err := h.questionService.Delete(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Question deleted successfully",
	})
}

// ListQuestions lists questions with filters
// @Summary List questions
// @Description Lists questions with optional filtering
// @Tags questions
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(10)
// @Param type query string false "Question type"
// @Param difficulty query string false "Difficulty level"
// @Param creator_id query uint false "Creator ID"
// @Success 200 {object} SuccessResponse{data=services.QuestionListResponse}
// @Failure 500 {object} ErrorResponse
// @Router /questions [get]
func (h *QuestionHandler) ListQuestions(c *gin.Context) {
	h.LogRequest(c, "Listing questions")

	filters := h.parseQuestionFilters(c)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	questions, err := h.questionService.List(c.Request.Context(), filters, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, questions)
}

// GetQuestionsByCreator lists questions by creator
// @Summary Get questions by creator
// @Description Lists questions created by a specific user
// @Tags questions
// @Accept json
// @Produce json
// @Param creator_id path uint true "Creator ID"
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(10)
// @Success 200 {object} SuccessResponse{data=services.QuestionListResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /questions/creator/{creator_id} [get]
func (h *QuestionHandler) GetQuestionsByCreator(c *gin.Context) {
	creatorID := ParseStringIDParam(c, "creator_id")
	if creatorID == "" {
		return
	}

	h.LogRequest(c, "Getting questions by creator", "creator_id", creatorID)

	filters := h.parseQuestionFilters(c)
	questions, err := h.questionService.GetByCreator(c.Request.Context(), creatorID, filters)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, questions)
}

// SearchQuestions searches questions
// @Summary Search questions
// @Description Searches questions by query string
// @Tags questions
// @Accept json
// @Produce json
// @Param q query string true "Search query"
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(10)
// @Success 200 {object} SuccessResponse{data=services.QuestionListResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /questions/search [get]
func (h *QuestionHandler) SearchQuestions(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Search query is required",
		})
		return
	}

	h.LogRequest(c, "Searching questions", "query", query)

	filters := h.parseQuestionFilters(c)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	questions, err := h.questionService.Search(c.Request.Context(), query, filters, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, questions)
}

// GetRandomQuestions gets random questions
// @Summary Get random questions
// @Description Gets random questions based on filters
// @Tags questions
// @Accept json
// @Produce json
// @Param count query int false "Number of questions" default(10)
// @Param type query string false "Question type"
// @Param difficulty query string false "Difficulty level"
// @Success 200 {object} SuccessResponse{data=[]models.Question}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /questions/random [get]
func (h *QuestionHandler) GetRandomQuestions(c *gin.Context) {
	h.LogRequest(c, "Getting random questions")

	filters := h.parseRandomQuestionFilters(c)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	questions, err := h.questionService.GetRandomQuestions(c.Request.Context(), filters, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, questions)
}

// CreateQuestionsBatch creates multiple questions in batch
// @Summary Create questions batch
// @Description Creates multiple questions in a single batch operation
// @Tags questions
// @Accept json
// @Produce json
// @Param questions body []services.CreateQuestionRequest true "Questions data"
// @Success 201 {object} SuccessResponse{data=[]services.QuestionResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /questions/batch [post]
func (h *QuestionHandler) CreateQuestionsBatch(c *gin.Context) {
	h.LogRequest(c, "Creating questions batch")

	var questions []*services.CreateQuestionRequest
	if err := c.ShouldBindJSON(&questions); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid request payload",
			Details: err.Error(),
		})
		return
	}

	if len(questions) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "At least one question is required",
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	results, errors := h.questionService.CreateBatch(c.Request.Context(), questions, userID.(string))

	// Check if there are any errors
	hasErrors := false
	for _, err := range errors {
		if err != nil {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Some questions failed to create",
			Details: errors,
		})
		return
	}

	c.JSON(http.StatusCreated, results)
}

// UpdateQuestionsBatch updates multiple questions in batch
// @Summary Update questions batch
// @Description Updates multiple questions in a single batch operation
// @Tags questions
// @Accept json
// @Produce json
// @Param updates body map[uint]services.UpdateQuestionRequest true "Question updates map"
// @Success 200 {object} SuccessResponse{data=map[uint]services.QuestionResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /questions/batch [put]
func (h *QuestionHandler) UpdateQuestionsBatch(c *gin.Context) {
	h.LogRequest(c, "Updating questions batch")

	var updates map[uint]*services.UpdateQuestionRequest
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid request payload",
			Details: err.Error(),
		})
		return
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "At least one question update is required",
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	results, errors := h.questionService.UpdateBatch(c.Request.Context(), updates, userID.(string))

	// Check if there are any errors
	hasErrors := false
	for _, err := range errors {
		if err != nil {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Some questions failed to update",
			Details: errors,
		})
		return
	}

	c.JSON(http.StatusOK, results)
}

// GetQuestionsByBank gets questions by question bank
// @Summary Get questions by bank
// @Description Gets questions from a specific question bank
// @Tags questions
// @Accept json
// @Produce json
// @Param bank_id path uint true "Bank ID"
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(10)
// @Success 200 {object} SuccessResponse{data=services.QuestionListResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /questions/bank/{bank_id} [get]
func (h *QuestionHandler) GetQuestionsByBank(c *gin.Context) {
	bankID := h.parseIDParam(c, "bank_id")
	if bankID == 0 {
		return
	}

	h.LogRequest(c, "Getting questions by bank", "bank_id", bankID)

	filters := h.parseQuestionFilters(c)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	questions, err := h.questionService.GetByBank(c.Request.Context(), bankID, filters, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, questions)
}

// AddQuestionToBank adds a question to a bank
// @Summary Add question to bank
// @Description Adds a question to a question bank
// @Tags questions
// @Accept json
// @Produce json
// @Param id path uint true "Question ID"
// @Param bank_id path uint true "Bank ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /questions/{id}/bank/{bank_id} [post]
func (h *QuestionHandler) AddQuestionToBank(c *gin.Context) {
	questionID := h.parseIDParam(c, "id")
	if questionID == 0 {
		return
	}

	bankID := h.parseIDParam(c, "bank_id")
	if bankID == 0 {
		return
	}

	h.LogRequest(c, "Adding question to bank", "question_id", questionID, "bank_id", bankID)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	err := h.questionService.AddToBank(c.Request.Context(), questionID, bankID, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Question added to bank successfully",
	})
}

// RemoveQuestionFromBank removes a question from a bank
// @Summary Remove question from bank
// @Description Removes a question from a question bank
// @Tags questions
// @Accept json
// @Produce json
// @Param id path uint true "Question ID"
// @Param bank_id path uint true "Bank ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /questions/{id}/bank/{bank_id} [delete]
func (h *QuestionHandler) RemoveQuestionFromBank(c *gin.Context) {
	questionID := h.parseIDParam(c, "id")
	if questionID == 0 {
		return
	}

	bankID := h.parseIDParam(c, "bank_id")
	if bankID == 0 {
		return
	}

	h.LogRequest(c, "Removing question from bank", "question_id", questionID, "bank_id", bankID)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	err := h.questionService.RemoveFromBank(c.Request.Context(), questionID, bankID, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Question removed from bank successfully",
	})
}

// GetQuestionStats retrieves question statistics
// @Summary Get question statistics
// @Description Retrieves statistics for a question
// @Tags questions
// @Accept json
// @Produce json
// @Param id path uint true "Question ID"
// @Success 200 {object} SuccessResponse{data=repositories.QuestionStats}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /questions/{id}/stats [get]
func (h *QuestionHandler) GetQuestionStats(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Getting question stats", "question_id", id)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	stats, err := h.questionService.GetStats(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetQuestionUsageStats retrieves question usage statistics
// @Summary Get question usage statistics
// @Description Retrieves usage statistics for questions by creator
// @Tags questions
// @Accept json
// @Produce json
// @Param creator_id path uint true "Creator ID"
// @Success 200 {object} SuccessResponse{data=repositories.QuestionUsageStats}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /questions/creator/{creator_id}/usage-stats [get]
func (h *QuestionHandler) GetQuestionUsageStats(c *gin.Context) {
	creatorID := ParseStringIDParam(c, "creator_id")
	if creatorID == "" {
		return
	}

	h.LogRequest(c, "Getting question usage stats", "creator_id", creatorID)

	stats, err := h.questionService.GetUsageStats(c.Request.Context(), creatorID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, stats)
}

// Helper methods

func (h *QuestionHandler) getUserID(c *gin.Context) string {
	userID, exists := c.Get("user_id")
	if !exists {
		return ""
	}
	if id, ok := userID.(string); ok {
		return id
	}
	return ""
}

func (h *QuestionHandler) parseIDParam(c *gin.Context, param string) uint {
	idStr := c.Param(param)
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid " + param,
			Details: err.Error(),
		})
		return 0
	}
	return uint(id)
}

func (h *QuestionHandler) parseIntQuery(c *gin.Context, param string, defaultValue int) int {
	valueStr := c.Query(param)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func (h *QuestionHandler) parseQuestionFilters(c *gin.Context) repositories.QuestionFilters {
	page := h.parseIntQuery(c, "page", 1)
	size := h.parseIntQuery(c, "size", 10)

	filters := repositories.QuestionFilters{
		Limit:  size,
		Offset: (page - 1) * size,
	}

	if questionType := c.Query("type"); questionType != "" {
		qType := models.QuestionType(questionType)
		filters.Type = &qType
	}

	if difficulty := c.Query("difficulty"); difficulty != "" {
		diffLevel := models.DifficultyLevel(difficulty)
		filters.Difficulty = &diffLevel
	}

	if creatorIDStr := c.Query("creator_id"); creatorIDStr != "" {
		filters.CreatedBy = &creatorIDStr
	}

	if categoryIDStr := c.Query("category_id"); categoryIDStr != "" {
		if categoryID, err := strconv.ParseUint(categoryIDStr, 10, 32); err == nil {
			id := uint(categoryID)
			filters.CategoryID = &id
		}
	}

	return filters
}

func (h *QuestionHandler) handleServiceError(c *gin.Context, err error) {
	// Handle custom error types first
	var validationErrors services.ValidationErrors
	if errors.As(err, &validationErrors) {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Validation failed",
			Details: validationErrors,
		})
		return
	}

	var businessRuleError *services.BusinessRuleError
	if errors.As(err, &businessRuleError) {
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{
			Message: businessRuleError.Message,
			Details: map[string]interface{}{
				"rule":    businessRuleError.Rule,
				"context": businessRuleError.Context,
			},
		})
		return
	}

	var permissionError *services.PermissionError
	if errors.As(err, &permissionError) {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Message: "Access denied",
			Details: map[string]interface{}{
				"resource": permissionError.Resource,
				"action":   permissionError.Action,
				"reason":   permissionError.Reason,
			},
		})
		return
	}

	// Handle specific question errors
	switch {
	case errors.Is(err, services.ErrQuestionNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Question not found",
		})
	case errors.Is(err, services.ErrQuestionAccessDenied):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Message: "Access denied to question",
		})
	case errors.Is(err, services.ErrQuestionInvalidType):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid question type",
		})
	case errors.Is(err, services.ErrQuestionInvalidContent):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid question content for type",
		})
	case errors.Is(err, services.ErrQuestionNotDeletable):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Question cannot be deleted - in use by assessments",
		})
	case errors.Is(err, services.ErrQuestionDuplicateOrder):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Question order already exists in assessment",
		})
	// Question Bank related errors
	case errors.Is(err, services.ErrQuestionBankNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Question bank not found",
		})
	case errors.Is(err, services.ErrQuestionBankAccessDenied):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Message: "Access denied to question bank",
		})
	case errors.Is(err, services.ErrQuestionBankNotDeletable):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Question bank cannot be deleted - has existing questions",
		})
	case errors.Is(err, services.ErrQuestionBankDuplicateName):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Question bank name already exists for this user",
		})
	case errors.Is(err, services.ErrQuestionBankShareExists):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Question bank already shared with this user",
		})
	case errors.Is(err, services.ErrQuestionBankNotShared):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Question bank is not shared with this user",
		})
	// Generic errors
	case errors.Is(err, services.ErrValidationFailed):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Validation failed",
			Details: err.Error(),
		})
	case errors.Is(err, services.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "Unauthorized access",
		})
	case errors.Is(err, services.ErrForbidden), errors.Is(err, services.ErrInsufficientPermissions):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Message: "Forbidden - insufficient permissions",
		})
	case errors.Is(err, services.ErrBadRequest):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Bad request",
		})
	case errors.Is(err, services.ErrConflict):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Resource conflict",
		})
	case errors.Is(err, services.ErrUserNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "User not found",
		})
	default:
		h.LogError(c, err, "Unexpected service error")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Message: "Internal server error",
		})
	}
}

func (h *QuestionHandler) parseRandomQuestionFilters(c *gin.Context) repositories.RandomQuestionFilters {
	count := h.parseIntQuery(c, "count", 10)

	filters := repositories.RandomQuestionFilters{
		Count: count,
	}

	if questionType := c.Query("type"); questionType != "" {
		qType := models.QuestionType(questionType)
		filters.Type = &qType
	}

	if difficulty := c.Query("difficulty"); difficulty != "" {
		diffLevel := models.DifficultyLevel(difficulty)
		filters.Difficulty = &diffLevel
	}

	if categoryIDStr := c.Query("category_id"); categoryIDStr != "" {
		if categoryID, err := strconv.ParseUint(categoryIDStr, 10, 32); err == nil {
			id := uint(categoryID)
			filters.CategoryID = &id
		}
	}

	return filters
}
