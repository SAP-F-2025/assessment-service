package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/services"
	"github.com/SAP-F-2025/assessment-service/internal/utils"
	"github.com/gin-gonic/gin"
)

type GroupHandler struct {
	BaseHandler
	service services.GroupService
}

func NewGroupHandler(service services.GroupService, logger utils.Logger) *GroupHandler {
	return &GroupHandler{
		BaseHandler: NewBaseHandler(logger),
		service:     service,
	}
}

// ===== CORE CRUD ENDPOINTS =====

// CreateGroup creates a new group (class) - Teachers only
// @Summary Create a new group (class)
// @Description Create a new group. Only teachers can create groups.
// @Tags groups
// @Accept json
// @Produce json
// @Param request body services.CreateGroupRequest true "Group creation request"
// @Success 201 {object} services.GroupResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - only teachers can create groups"
// @Failure 409 {object} ErrorResponse "Conflict - group name already exists"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups [post]
func (h *GroupHandler) CreateGroup(c *gin.Context) {
	var req services.CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid request payload",
			Details: err.Error(),
		})
		return
	}

	// Get user ID from JWT token (middleware should set this)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	response, err := h.service.Create(c.Request.Context(), &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, response)
}

// GetGroup retrieves a group by ID
// @Summary Get a group by ID
// @Description Retrieve a group with its basic information
// @Tags groups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Success 200 {object} services.GroupResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - not a member"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/{id} [get]
func (h *GroupHandler) GetGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
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

	response, err := h.service.GetByID(c.Request.Context(), uint(id), userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetGroupWithMembers retrieves a group by ID with members
// @Summary Get a group by ID with members
// @Description Retrieve a group with its members list
// @Tags groups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Success 200 {object} services.GroupResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - not a member"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/{id}/details [get]
func (h *GroupHandler) GetGroupWithMembers(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
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

	response, err := h.service.GetByIDWithMembers(c.Request.Context(), uint(id), userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// UpdateGroup updates a group (class owner only)
// @Summary Update a group
// @Description Update group information. Only the class owner can update.
// @Tags groups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Param request body services.UpdateGroupRequest true "Group update request"
// @Success 200 {object} services.GroupResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - not the owner"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/{id} [put]
func (h *GroupHandler) UpdateGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid group ID",
		})
		return
	}

	var req services.UpdateGroupRequest
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

	response, err := h.service.Update(c.Request.Context(), uint(id), &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// DeleteGroup deletes a group (class owner only)
// @Summary Delete a group
// @Description Delete a group. Only the class owner can delete.
// @Tags groups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - not the owner"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/{id} [delete]
func (h *GroupHandler) DeleteGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
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

	err = h.service.Delete(c.Request.Context(), uint(id), userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ===== LIST AND SEARCH ENDPOINTS =====

// ListGroups lists groups with pagination and filters
// @Summary List groups
// @Description List groups the user has access to with pagination and filters
// @Tags groups
// @Accept json
// @Produce json
// @Param query query string false "Search query"
// @Param type query string false "Filter by type"
// @Param limit query int false "Results per page (default 10, max 100)"
// @Param offset query int false "Offset for pagination"
// @Success 200 {object} services.GroupListResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups [get]
func (h *GroupHandler) ListGroups(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	// Parse query parameters
	filters := repositories.GroupFilters{
		Query:  c.Query("query"),
		Type:   c.Query("type"),
		Limit:  10,
		Offset: 0,
	}

	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			filters.Limit = l
		}
	}

	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			filters.Offset = o
		}
	}

	response, err := h.service.List(c.Request.Context(), filters, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetMyGroups gets groups created by the current user
// @Summary Get my groups
// @Description Get groups created by the current user (teacher)
// @Tags groups
// @Accept json
// @Produce json
// @Success 200 {object} services.GroupListResponse
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/my [get]
func (h *GroupHandler) GetMyGroups(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	response, err := h.service.GetByCreator(c.Request.Context(), userID.(string), repositories.GroupFilters{})
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetMyMemberships gets groups where the current user is a member
// @Summary Get my group memberships
// @Description Get groups where the current user is a member (student or teacher)
// @Tags groups
// @Accept json
// @Produce json
// @Success 200 {object} services.GroupListResponse
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/memberships [get]
func (h *GroupHandler) GetMyMemberships(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	response, err := h.service.GetByMember(c.Request.Context(), userID.(string), repositories.GroupFilters{})
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// ===== MEMBER MANAGEMENT ENDPOINTS =====

// GetMembers gets members of a group
// @Summary Get group members
// @Description Get all members of a group
// @Tags groups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Success 200 {array} services.GroupMemberResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - not a member"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/{id}/members [get]
func (h *GroupHandler) GetMembers(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
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

	members, err := h.service.GetMembers(c.Request.Context(), uint(id), userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, members)
}

// AddMember adds a member to a group
// @Summary Add member to group
// @Description Add a user to a group. Owner and teacher members can add members.
// @Tags groups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Param request body services.AddGroupMemberRequest true "Add member request"
// @Success 201 "Created"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - cannot manage members"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 409 {object} ErrorResponse "Conflict - already a member"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/{id}/members [post]
func (h *GroupHandler) AddMember(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid group ID",
		})
		return
	}

	var req services.AddGroupMemberRequest
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

	err = h.service.AddMember(c.Request.Context(), uint(id), &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse{
		Message: "Member added successfully",
	})
}

// RemoveMember removes a member from a group
// @Summary Remove member from group
// @Description Remove a user from a group. Owner and teacher members can remove students. Only owner can remove teachers.
// @Tags groups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Param userId path string true "User ID to remove"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - cannot manage members"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/{id}/members/{userId} [delete]
func (h *GroupHandler) RemoveMember(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid group ID",
		})
		return
	}

	memberUserID := c.Param("userId")
	if memberUserID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid user ID",
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

	err = h.service.RemoveMember(c.Request.Context(), uint(id), memberUserID, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// UpdateMemberRole updates a member's role in a group (owner only)
// @Summary Update member role
// @Description Update a member's role in a group. Only the class owner can update roles.
// @Tags groups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Param userId path string true "User ID"
// @Param request body services.UpdateMemberRoleRequest true "Update role request"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - only owner can update roles"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/{id}/members/{userId}/role [put]
func (h *GroupHandler) UpdateMemberRole(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid group ID",
		})
		return
	}

	memberUserID := c.Param("userId")
	if memberUserID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid user ID",
		})
		return
	}

	var req services.UpdateMemberRoleRequest
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

	err = h.service.UpdateMemberRole(c.Request.Context(), uint(id), memberUserID, &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Member role updated successfully",
	})
}

// ===== INVITE MANAGEMENT ENDPOINTS =====

// CreateInviteLink creates an invite link for a group
// @Summary Create invite link
// @Description Create an invite link for a group. Only owner or co-owner can create invites.
// @Tags groups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Param request body services.CreateInviteLinkRequest false "Invite link settings"
// @Success 201 {object} services.GroupInviteResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - not owner or co-owner"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/{id}/invites/link [post]
func (h *GroupHandler) CreateInviteLink(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid group ID",
		})
		return
	}

	var req services.CreateInviteLinkRequest
	// Request body is optional
	_ = c.ShouldBindJSON(&req)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	response, err := h.service.CreateInviteLink(c.Request.Context(), uint(id), &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, response)
}

// CreateInviteCode creates an invite code for a group
// @Summary Create invite code
// @Description Create a short invite code for a group. Only owner or co-owner can create invites.
// @Tags groups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Param request body services.CreateInviteCodeRequest false "Invite code settings"
// @Success 201 {object} services.GroupInviteResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - not owner or co-owner"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/{id}/invites/code [post]
func (h *GroupHandler) CreateInviteCode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid group ID",
		})
		return
	}

	var req services.CreateInviteCodeRequest
	// Request body is optional
	_ = c.ShouldBindJSON(&req)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	response, err := h.service.CreateInviteCode(c.Request.Context(), uint(id), &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, response)
}

// GetGroupInvites gets all invites for a group
// @Summary Get group invites
// @Description Get all invites for a group. Only owner or co-owner can view invites.
// @Tags groups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Success 200 {array} services.GroupInviteResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - not owner or co-owner"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/{id}/invites [get]
func (h *GroupHandler) GetGroupInvites(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
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

	response, err := h.service.GetGroupInvites(c.Request.Context(), uint(id), userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// DeleteInvite deletes an invite
// @Summary Delete invite
// @Description Delete an invite. Only owner or co-owner can delete invites.
// @Tags groups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Param inviteId path int true "Invite ID"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - not owner or co-owner"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/{id}/invites/{inviteId} [delete]
func (h *GroupHandler) DeleteInvite(c *gin.Context) {
	inviteID, err := strconv.ParseUint(c.Param("inviteId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid invite ID",
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

	err = h.service.DeleteInvite(c.Request.Context(), uint(inviteID), userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// RegenerateInvite regenerates an invite token/code
// @Summary Regenerate invite
// @Description Regenerate an invite's token or code. Only owner or co-owner can regenerate invites.
// @Tags groups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Param inviteId path int true "Invite ID"
// @Success 200 {object} services.GroupInviteResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - not owner or co-owner"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/{id}/invites/{inviteId}/regenerate [post]
func (h *GroupHandler) RegenerateInvite(c *gin.Context) {
	inviteID, err := strconv.ParseUint(c.Param("inviteId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid invite ID",
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

	response, err := h.service.RegenerateInvite(c.Request.Context(), uint(inviteID), userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// ===== JOIN VIA INVITE ENDPOINTS =====

// JoinViaLink allows joining a group via invite link
// @Summary Join group via invite link
// @Description Join a group using an invite link token.
// @Tags groups
// @Accept json
// @Produce json
// @Param token path string true "Invite token"
// @Success 200 {object} services.GroupResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 404 {object} ErrorResponse "Invite not found"
// @Failure 409 {object} ErrorResponse "Already a member"
// @Failure 410 {object} ErrorResponse "Invite expired or exhausted"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/join/link/{token} [post]
func (h *GroupHandler) JoinViaLink(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid invite token",
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

	response, err := h.service.JoinViaLink(c.Request.Context(), token, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// JoinViaCode allows joining a group via invite code
// @Summary Join group via invite code
// @Description Join a group using a short invite code.
// @Tags groups
// @Accept json
// @Produce json
// @Param request body services.JoinViaCodeRequest true "Join via code request"
// @Success 200 {object} services.GroupResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 404 {object} ErrorResponse "Invite not found"
// @Failure 409 {object} ErrorResponse "Already a member"
// @Failure 410 {object} ErrorResponse "Invite expired or exhausted"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/join/code [post]
func (h *GroupHandler) JoinViaCode(c *gin.Context) {
	var req services.JoinViaCodeRequest
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

	response, err := h.service.JoinViaCode(c.Request.Context(), req.Code, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// ===== LEAVE GROUP ENDPOINT =====

// LeaveGroup allows a member to leave a group
// @Summary Leave group
// @Description Leave a group. Owners cannot leave their own groups.
// @Tags groups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse "Bad request - owner cannot leave"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 404 {object} ErrorResponse "Not a member"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /groups/{id}/leave [delete]
func (h *GroupHandler) LeaveGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
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

	err = h.service.LeaveGroup(c.Request.Context(), uint(id), userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ===== ERROR HANDLING =====

func (h *GroupHandler) handleServiceError(c *gin.Context, err error) {
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
	case errors.Is(err, services.ErrGroupNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Group not found",
		})
	case errors.Is(err, services.ErrGroupDuplicateName):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Group with this name already exists",
		})
	case errors.Is(err, services.ErrGroupMemberExists):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "User is already a member of this group",
		})
	case errors.Is(err, services.ErrGroupMemberNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "User is not a member of this group",
		})
	case errors.Is(err, services.ErrGroupCannotRemoveOwner):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Cannot remove group owner",
		})
	case errors.Is(err, services.ErrGroupOwnerCannotLeave):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Group owner cannot leave their own group",
		})
	case errors.Is(err, services.ErrGroupAccessDenied):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Message: "Access denied to group",
		})
	case errors.Is(err, services.ErrGroupInviteNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Invite not found",
		})
	case errors.Is(err, services.ErrGroupInviteExpired):
		c.JSON(http.StatusGone, ErrorResponse{
			Message: "Invite has expired",
		})
	case errors.Is(err, services.ErrGroupInviteExhausted):
		c.JSON(http.StatusGone, ErrorResponse{
			Message: "Invite has reached maximum uses",
		})
	case errors.Is(err, services.ErrGroupInviteInvalid):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid invite",
		})
	case errors.Is(err, services.ErrValidationFailed):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Validation failed",
			Details: err.Error(),
		})
	case errors.Is(err, services.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "Unauthorized",
		})
	case errors.Is(err, services.ErrForbidden):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Message: "Forbidden",
		})
	case errors.Is(err, services.ErrGroupInvalidRole):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid group member role",
		})
	default:
		// Log unexpected errors
		h.LogError(c, err, "Unexpected error in group handler")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Message: "Internal server error",
		})
	}
}
