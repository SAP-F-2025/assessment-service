package handlers

import (
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/utils"
	"github.com/gin-gonic/gin"
)

// ===== COMMON RESPONSE STRUCTURES =====

// ErrorResponse represents an error response
type ErrorResponse struct {
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
	Code    string      `json:"code,omitempty"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ValidationErrorResponse represents validation error details
type ValidationErrorResponse struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// ===== REQUEST STRUCTURES =====

// UpdateSharePermissionsRequest for updating question bank share permissions
type UpdateSharePermissionsRequest struct {
	CanEdit   bool `json:"can_edit"`
	CanDelete bool `json:"can_delete"`
}

// RemoveQuestionsRequest for removing questions from a bank
type RemoveQuestionsRequest struct {
	QuestionIDs []uint `json:"question_ids" validate:"required,min=1"`
}

// ===== BASE HANDLER STRUCT =====

// BaseHandler provides common logging functionality for all handlers
type BaseHandler struct {
	logger utils.Logger
}

// NewBaseHandler creates a new base handler with logging capability
func NewBaseHandler(logger utils.Logger) BaseHandler {
	return BaseHandler{
		logger: logger,
	}
}

// LogRequest logs incoming HTTP requests with context information
func (h *BaseHandler) LogRequest(c *gin.Context, message string, additionalFields ...interface{}) {
	start := time.Now()

	// Extract user info if available
	userID := h.extractUserID(c)
	requestID := c.GetHeader("X-Request-ID")

	fields := []interface{}{
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"remote_addr", c.ClientIP(),
		"user_agent", c.Request.UserAgent(),
		"request_id", requestID,
		"user_id", userID,
		"timestamp", start.Format(time.RFC3339),
	}

	// Add any additional fields provided
	fields = append(fields, additionalFields...)

	h.logger.Info(message, fields...)
}

// LogResponse logs HTTP responses with timing and status information
func (h *BaseHandler) LogResponse(c *gin.Context, statusCode int, message string, additionalFields ...interface{}) {
	duration := time.Since(time.Now()) // This would be calculated from request start time in real implementation

	fields := []interface{}{
		"status_code", statusCode,
		"duration", duration.String(),
		"request_id", c.GetHeader("X-Request-ID"),
	}

	// Add any additional fields provided
	fields = append(fields, additionalFields...)

	h.logger.LogRequest(c.Request.Method, c.Request.URL.Path, statusCode, duration.String(), fields...)
}

// LogError logs error details with context information
func (h *BaseHandler) LogError(c *gin.Context, err error, message string, additionalFields ...interface{}) {
	requestID := c.GetHeader("X-Request-ID")
	userID := h.extractUserID(c)

	fields := []interface{}{
		"request_id", requestID,
		"user_id", userID,
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
	}

	// Add any additional fields provided
	fields = append(fields, additionalFields...)

	h.logger.LogError(err, message, fields...)
}

// LogDebug logs debug information with context
func (h *BaseHandler) LogDebug(c *gin.Context, message string, additionalFields ...interface{}) {
	requestID := c.GetHeader("X-Request-ID")
	userID := h.extractUserID(c)

	fields := []interface{}{
		"request_id", requestID,
		"user_id", userID,
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
	}

	// Add any additional fields provided
	fields = append(fields, additionalFields...)

	h.logger.Debug(message, fields...)
}

// LogInfo logs informational messages with context
func (h *BaseHandler) LogInfo(c *gin.Context, message string, additionalFields ...interface{}) {
	requestID := c.GetHeader("X-Request-ID")
	userID := h.extractUserID(c)

	fields := []interface{}{
		"request_id", requestID,
		"user_id", userID,
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
	}

	// Add any additional fields provided
	fields = append(fields, additionalFields...)

	h.logger.Info(message, fields...)
}

// LogWarn logs warning messages with context
func (h *BaseHandler) LogWarn(c *gin.Context, message string, additionalFields ...interface{}) {
	requestID := c.GetHeader("X-Request-ID")
	userID := h.extractUserID(c)

	fields := []interface{}{
		"request_id", requestID,
		"user_id", userID,
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
	}

	// Add any additional fields provided
	fields = append(fields, additionalFields...)

	h.logger.Warn(message, fields...)
}

// Helper method to extract user ID from context
func (h *BaseHandler) extractUserID(c *gin.Context) interface{} {
	if userID, exists := c.Get("user_id"); exists {
		return userID
	}
	return nil
}

// RespondWithError sends a consistent error response and logs it
func (h *BaseHandler) RespondWithError(c *gin.Context, statusCode int, message string, err error, details ...interface{}) {
	errorResp := ErrorResponse{
		Message: message,
	}

	if len(details) > 0 {
		errorResp.Details = details[0]
	}

	// Log the error with context
	if err != nil {
		h.LogError(c, err, message, "status_code", statusCode)
	} else {
		h.LogWarn(c, message, "status_code", statusCode)
	}

	c.JSON(statusCode, errorResp)
}

// RespondWithSuccess sends a consistent success response and logs it
func (h *BaseHandler) RespondWithSuccess(c *gin.Context, statusCode int, message string, data interface{}, additionalFields ...interface{}) {
	successResp := SuccessResponse{
		Message: message,
		Data:    data,
	}

	// Log the successful response
	fields := []interface{}{"status_code", statusCode}
	fields = append(fields, additionalFields...)
	h.LogInfo(c, message, fields...)

	c.JSON(statusCode, successResp)
}

// ===== HELPER IMPORTS =====

// Import models to make them available for type references
var (
	_ models.QuestionType    = ""
	_ models.DifficultyLevel = ""
)
