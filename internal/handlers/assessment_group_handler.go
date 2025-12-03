package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/SAP-F-2025/assessment-service/internal/services"
	"github.com/SAP-F-2025/assessment-service/internal/utils"
	"github.com/gin-gonic/gin"
)

type AssessmentGroupHandler struct {
	BaseHandler
	service services.AssessmentGroupService
}

func NewAssessmentGroupHandler(service services.AssessmentGroupService, logger utils.Logger) *AssessmentGroupHandler {
	return &AssessmentGroupHandler{
		BaseHandler: NewBaseHandler(logger),
		service:     service,
	}
}

// ===== ASSIGNMENT ENDPOINTS =====

// AssignToGroups assigns an assessment to multiple groups
// @Summary Assign assessment to groups
// @Description Assign an assessment to one or more groups. Requires teacher permissions.
// @Tags assessment-groups
// @Accept json
// @Produce json
// @Param id path int true "Assessment ID"
// @Param request body services.AssignAssessmentToGroupsRequest true "Assignment request"
// @Success 200 {object} MessageResponse "Success"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /assessments/{id}/groups [post]
func (h *AssessmentGroupHandler) AssignToGroups(c *gin.Context) {
	assessmentID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid assessment ID",
		})
		return
	}

	var req services.AssignAssessmentToGroupsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid request payload",
			Details: err.Error(),
		})
		return
	}

	// Get user ID from JWT token
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	if err := h.service.AssignToGroups(c.Request.Context(), uint(assessmentID), &req, userID.(string)); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Assessment assigned to groups successfully",
	})
}

// UnassignFromGroups removes assessment from groups
// @Summary Unassign assessment from groups
// @Description Remove an assessment from one or more groups. Requires teacher permissions.
// @Tags assessment-groups
// @Accept json
// @Produce json
// @Param id path int true "Assessment ID"
// @Param request body services.UnassignAssessmentFromGroupsRequest true "Unassignment request"
// @Success 200 {object} MessageResponse "Success"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /assessments/{id}/groups [delete]
func (h *AssessmentGroupHandler) UnassignFromGroups(c *gin.Context) {
	assessmentID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid assessment ID",
		})
		return
	}

	var req services.UnassignAssessmentFromGroupsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid request payload",
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

	if err := h.service.UnassignFromGroups(c.Request.Context(), uint(assessmentID), &req, userID.(string)); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Assessment unassigned from groups successfully",
	})
}

// ===== QUERY ENDPOINTS =====

// GetAssignedGroups retrieves groups assigned to an assessment
// @Summary Get assigned groups
// @Description Get all groups that an assessment is assigned to
// @Tags assessment-groups
// @Accept json
// @Produce json
// @Param id path int true "Assessment ID"
// @Success 200 {object} services.AssessmentGroupAssignmentResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /assessments/{id}/groups [get]
func (h *AssessmentGroupHandler) GetAssignedGroups(c *gin.Context) {
	assessmentID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid assessment ID",
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

	response, err := h.service.GetAssignedGroups(c.Request.Context(), uint(assessmentID), userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetGroupAssessments retrieves assessments assigned to a group
// @Summary Get group assessments
// @Description Get all assessments assigned to a specific group
// @Tags assessment-groups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Success 200 {object} services.GroupAssessmentListResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/{id}/assessments [get]
func (h *AssessmentGroupHandler) GetGroupAssessments(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid group ID",
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

	response, err := h.service.GetGroupAssessments(c.Request.Context(), uint(groupID), userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// ===== ERROR HANDLING =====

func (h *AssessmentGroupHandler) handleServiceError(c *gin.Context, err error) {
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

	// Handle standard service errors
	switch {
	case errors.Is(err, services.ErrAssessmentNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Assessment not found",
		})
	case errors.Is(err, services.ErrGroupNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Group not found",
		})
	case errors.Is(err, services.ErrAssessmentGroupNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Assessment-group assignment not found",
		})
	case errors.Is(err, services.ErrAssessmentGroupAlreadyAssigned):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Assessment already assigned to group",
		})
	case errors.Is(err, services.ErrAssessmentGroupNotAssigned):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Assessment not assigned to this group",
		})
	case errors.Is(err, services.ErrCannotAssignInActiveAssessment):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Cannot assign inactive assessment to groups",
		})
	case errors.Is(err, services.ErrValidationFailed):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Validation failed",
			Details: err.Error(),
		})
	default:
		h.logger.Error("Unhandled error in assessment-group handler", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Message: "An unexpected error occurred",
		})
	}
}
