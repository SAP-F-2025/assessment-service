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
	"github.com/gin-gonic/gin"
)

type GradingHandler struct {
	BaseHandler
	gradingService services.GradingService
	validator      *utils.Validator
}

type GradeAnswerRequest struct {
	Score    float64 `json:"score" validate:"required,min=0,max=100"`
	Feedback *string `json:"feedback"`
}

type GradeMultipleAnswersRequest struct {
	Grades []repositories.AnswerGrade `json:"grades" validate:"required,min=1,dive"`
}

func NewGradingHandler(
	gradingService services.GradingService,
	validator *utils.Validator,
	logger utils.Logger,
) *GradingHandler {
	return &GradingHandler{
		BaseHandler:    NewBaseHandler(logger),
		gradingService: gradingService,
		validator:      validator,
	}
}

// GradeAnswer grades a specific answer manually
// @Summary Grade answer
// @Description Manually grades a specific answer
// @Tags grading
// @Accept json
// @Produce json
// @Param answer_id path uint true "Answer ID"
// @Param grade body GradeAnswerRequest true "Grading data"
// @Success 200 {object} SuccessResponse{data=services.GradingResult}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /grading/answers/{answer_id} [post]
func (h *GradingHandler) GradeAnswer(c *gin.Context) {
	answerID := h.parseIDParam(c, "answer_id")
	if answerID == 0 {
		return
	}

	h.LogRequest(c, "Grading answer", "answer_id", answerID)

	var req GradeAnswerRequest
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
	result, err := h.gradingService.GradeAnswer(c.Request.Context(), answerID, req.Score, req.Feedback, userID.(uint))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// GradeAttempt grades an entire attempt manually
// @Summary Grade attempt
// @Description Manually grades an entire assessment attempt
// @Tags grading
// @Accept json
// @Produce json
// @Param attempt_id path uint true "Attempt ID"
// @Success 200 {object} SuccessResponse{data=services.AttemptGradingResult}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /grading/attempts/{attempt_id} [post]
func (h *GradingHandler) GradeAttempt(c *gin.Context) {
	attemptID := h.parseIDParam(c, "attempt_id")
	if attemptID == 0 {
		return
	}

	h.LogRequest(c, "Grading attempt", "attempt_id", attemptID)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	result, err := h.gradingService.GradeAttempt(c.Request.Context(), attemptID, userID.(uint))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// GradeMultipleAnswers grades multiple answers in batch
// @Summary Grade multiple answers
// @Description Grades multiple answers in a single batch operation
// @Tags grading
// @Accept json
// @Produce json
// @Param grades body GradeMultipleAnswersRequest true "Multiple grades data"
// @Success 200 {object} SuccessResponse{data=[]services.GradingResult}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /grading/answers/batch [post]
func (h *GradingHandler) GradeMultipleAnswers(c *gin.Context) {
	h.LogRequest(c, "Grading multiple answers")

	var req GradeMultipleAnswersRequest
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
	results, err := h.gradingService.GradeMultipleAnswers(c.Request.Context(), req.Grades, userID.(uint))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, results)
}

// AutoGradeAnswer automatically grades a specific answer
// @Summary Auto-grade answer
// @Description Automatically grades a specific answer using predefined rules
// @Tags grading
// @Accept json
// @Produce json
// @Param answer_id path uint true "Answer ID"
// @Success 200 {object} SuccessResponse{data=services.GradingResult}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /grading/answers/{answer_id}/auto [post]
func (h *GradingHandler) AutoGradeAnswer(c *gin.Context) {
	answerID := h.parseIDParam(c, "answer_id")
	if answerID == 0 {
		return
	}

	h.LogRequest(c, "Auto-grading answer", "answer_id", answerID)

	result, err := h.gradingService.AutoGradeAnswer(c.Request.Context(), answerID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// AutoGradeAttempt automatically grades an entire attempt
// @Summary Auto-grade attempt
// @Description Automatically grades an entire assessment attempt
// @Tags grading
// @Accept json
// @Produce json
// @Param attempt_id path uint true "Attempt ID"
// @Success 200 {object} SuccessResponse{data=services.AttemptGradingResult}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /grading/attempts/{attempt_id}/auto [post]
func (h *GradingHandler) AutoGradeAttempt(c *gin.Context) {
	attemptID := h.parseIDParam(c, "attempt_id")
	if attemptID == 0 {
		return
	}

	h.LogRequest(c, "Auto-grading attempt", "attempt_id", attemptID)

	result, err := h.gradingService.AutoGradeAttempt(c.Request.Context(), attemptID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// AutoGradeAssessment automatically grades all attempts for an assessment
// @Summary Auto-grade assessment
// @Description Automatically grades all attempts for a specific assessment
// @Tags grading
// @Accept json
// @Produce json
// @Param assessment_id path uint true "Assessment ID"
// @Success 200 {object} SuccessResponse{data=map[uint]services.AttemptGradingResult}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /grading/assessments/{assessment_id}/auto [post]
func (h *GradingHandler) AutoGradeAssessment(c *gin.Context) {
	assessmentID := h.parseIDParam(c, "assessment_id")
	if assessmentID == 0 {
		return
	}

	h.LogRequest(c, "Auto-grading assessment", "assessment_id", assessmentID)

	results, err := h.gradingService.AutoGradeAssessment(c.Request.Context(), assessmentID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, results)
}

// CalculateScore calculates score for a specific answer
// @Summary Calculate score
// @Description Calculates the score for a specific answer without saving it
// @Tags grading
// @Accept json
// @Produce json
// @Param question_type query string true "Question type"
// @Param question_content body json.RawMessage true "Question content JSON"
// @Param student_answer body json.RawMessage true "Student answer JSON"
// @Success 200 {object} SuccessResponse{data=map[string]interface{}}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /grading/calculate-score [post]
func (h *GradingHandler) CalculateScore(c *gin.Context) {
	questionType := c.Query("question_type")
	if questionType == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Question type is required",
		})
		return
	}

	h.LogRequest(c, "Calculating score", "question_type", questionType)

	var body struct {
		QuestionContent json.RawMessage `json:"question_content" validate:"required"`
		StudentAnswer   json.RawMessage `json:"student_answer" validate:"required"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid request payload",
			Details: err.Error(),
		})
		return
	}

	if err := h.validator.Validate(&body); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Validation failed",
			Details: err.Error(),
		})
		return
	}

	// Convert questionType string to models.QuestionType
	// Note: You might need to import the models package and handle type conversion
	score, isCorrect, err := h.gradingService.CalculateScore(c.Request.Context(),
		models.QuestionType(questionType), body.QuestionContent, body.StudentAnswer)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	result := map[string]interface{}{
		"score":      score,
		"is_correct": isCorrect,
	}

	c.JSON(http.StatusOK, result)
}

// GenerateFeedback generates feedback for an answer
// @Summary Generate feedback
// @Description Generates feedback for a specific answer without saving it
// @Tags grading
// @Accept json
// @Produce json
// @Param question_type query string true "Question type"
// @Param is_correct query bool true "Is answer correct"
// @Param question_content body json.RawMessage true "Question content JSON"
// @Param student_answer body json.RawMessage true "Student answer JSON"
// @Success 200 {object} SuccessResponse{data=map[string]interface{}}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /grading/generate-feedback [post]
func (h *GradingHandler) GenerateFeedback(c *gin.Context) {
	questionType := c.Query("question_type")
	if questionType == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Question type is required",
		})
		return
	}

	isCorrectStr := c.Query("is_correct")
	if isCorrectStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "is_correct parameter is required",
		})
		return
	}

	isCorrect, err := strconv.ParseBool(isCorrectStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid is_correct value",
			Details: err.Error(),
		})
		return
	}

	h.LogRequest(c, "Generating feedback", "question_type", questionType, "is_correct", isCorrect)

	var body struct {
		QuestionContent json.RawMessage `json:"question_content" validate:"required"`
		StudentAnswer   json.RawMessage `json:"student_answer" validate:"required"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid request payload",
			Details: err.Error(),
		})
		return
	}

	if err := h.validator.Validate(&body); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Validation failed",
			Details: err.Error(),
		})
		return
	}

	// Convert questionType string to models.QuestionType
	feedback, err := h.gradingService.GenerateFeedback(c.Request.Context(),
		models.QuestionType(questionType), body.QuestionContent, body.StudentAnswer, isCorrect)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	result := map[string]interface{}{
		"feedback": feedback,
	}

	c.JSON(http.StatusOK, result)
}

// ReGradeQuestion re-grades all answers for a specific question
// @Summary Re-grade question
// @Description Re-grades all answers for a specific question
// @Tags grading
// @Accept json
// @Produce json
// @Param question_id path uint true "Question ID"
// @Success 200 {object} SuccessResponse{data=[]services.GradingResult}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /grading/questions/{question_id}/regrade [post]
func (h *GradingHandler) ReGradeQuestion(c *gin.Context) {
	questionID := h.parseIDParam(c, "question_id")
	if questionID == 0 {
		return
	}

	h.LogRequest(c, "Re-grading question", "question_id", questionID)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	results, err := h.gradingService.ReGradeQuestion(c.Request.Context(), questionID, userID.(uint))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, results)
}

// ReGradeAssessment re-grades all attempts for an assessment
// @Summary Re-grade assessment
// @Description Re-grades all attempts for a specific assessment
// @Tags grading
// @Accept json
// @Produce json
// @Param assessment_id path uint true "Assessment ID"
// @Success 200 {object} SuccessResponse{data=map[uint]services.AttemptGradingResult}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /grading/assessments/{assessment_id}/regrade [post]
func (h *GradingHandler) ReGradeAssessment(c *gin.Context) {
	assessmentID := h.parseIDParam(c, "assessment_id")
	if assessmentID == 0 {
		return
	}

	h.LogRequest(c, "Re-grading assessment", "assessment_id", assessmentID)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	results, err := h.gradingService.ReGradeAssessment(c.Request.Context(), assessmentID, userID.(uint))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, results)
}

// GetGradingOverview gets grading overview for an assessment
// @Summary Get grading overview
// @Description Gets grading statistics and overview for an assessment
// @Tags grading
// @Accept json
// @Produce json
// @Param assessment_id path uint true "Assessment ID"
// @Success 200 {object} SuccessResponse{data=repositories.GradingStats}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /grading/assessments/{assessment_id}/overview [get]
func (h *GradingHandler) GetGradingOverview(c *gin.Context) {
	assessmentID := h.parseIDParam(c, "assessment_id")
	if assessmentID == 0 {
		return
	}

	h.LogRequest(c, "Getting grading overview", "assessment_id", assessmentID)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	overview, err := h.gradingService.GetGradingOverview(c.Request.Context(), assessmentID, userID.(uint))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, overview)
}

// Helper methods

func (h *GradingHandler) getUserID(c *gin.Context) uint {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0
	}
	if id, ok := userID.(uint); ok {
		return id
	}
	return 0
}

func (h *GradingHandler) parseIDParam(c *gin.Context, param string) uint {
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

func (h *GradingHandler) handleServiceError(c *gin.Context, err error) {
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

	// Handle specific grading errors
	switch {
	case errors.Is(err, services.ErrGradingNotAllowed):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Message: "Grading not allowed for this question type",
		})
	case errors.Is(err, services.ErrGradingAlreadyCompleted):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Answer already graded",
		})
	case errors.Is(err, services.ErrGradingInvalidScore):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid score value",
		})
	case errors.Is(err, services.ErrGradingPermissionDenied):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Message: "Permission denied for grading",
		})
	// Related entity errors
	case errors.Is(err, services.ErrAttemptNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Attempt not found",
		})
	case errors.Is(err, services.ErrQuestionNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Question not found",
		})
	case errors.Is(err, services.ErrAssessmentNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Assessment not found",
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
