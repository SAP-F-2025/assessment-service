package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/validator"
	"gorm.io/gorm"
)

type groupService struct {
	repo      repositories.Repository
	db        *gorm.DB
	logger    *slog.Logger
	validator *validator.Validator
}

func NewGroupService(repo repositories.Repository, db *gorm.DB, logger *slog.Logger, validator *validator.Validator) GroupService {
	return &groupService{
		repo:      repo,
		db:        db,
		logger:    logger,
		validator: validator,
	}
}

// ===== CORE CRUD OPERATIONS =====

func (s *groupService) Create(ctx context.Context, req *CreateGroupRequest, creatorID string) (*GroupResponse, error) {
	s.logger.Info("Creating group", "creator_id", creatorID, "name", req.Name)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Verify creator exists
	creator, err := s.repo.User().GetByID(ctx, creatorID)
	if err != nil {
		return nil, fmt.Errorf("failed to get creator user: %w", err)
	}

	// Check if group with same name exists
	exists, err := s.repo.Group().ExistsByName(ctx, nil, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check group name uniqueness: %w", err)
	}
	if exists {
		return nil, ErrGroupDuplicateName
	}

	// Create group in transaction (to also add creator as owner member)
	var group *models.Group
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Create group
		group = &models.Group{
			Name:        req.Name,
			DisplayName: req.DisplayName,
			Description: "",
			Type:        req.Type,
			CreatedBy:   creatorID,
		}

		if req.Description != nil {
			group.Description = *req.Description
		}

		if req.Type == "" {
			group.Type = "class"
		}

		if err := s.repo.Group().Create(ctx, tx, group); err != nil {
			return fmt.Errorf("failed to create group: %w", err)
		}

		// Add creator as owner member
		// This allows the creator to be seen in the members list with their role
		ownerMember := &models.GroupMember{
			GroupID: group.ID,
			UserID:  creatorID,
			Role:    models.GroupMemberRoleOwner,
		}

		// Determine display role based on user's system role
		// If creator is a student, they become owner of a student-created group
		// If creator is a teacher, they become owner of a teacher-created group
		_ = creator // creator is used for logging/validation if needed

		if err := s.repo.Group().AddMember(ctx, tx, ownerMember); err != nil {
			return fmt.Errorf("failed to add creator as owner member: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Group created successfully", "group_id", group.ID, "creator", creatorID)

	return s.buildGroupResponse(ctx, group, creatorID), nil
}

func (s *groupService) GetByID(ctx context.Context, id uint, userID string) (*GroupResponse, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, id, "group", "read", "not a member of this class")
	}

	// Get group
	group, err := s.repo.Group().GetByID(ctx, nil, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrGroupNotFound
		}
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	return s.buildGroupResponse(ctx, group, userID), nil
}

func (s *groupService) GetByIDWithMembers(ctx context.Context, id uint, userID string) (*GroupResponse, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, id, "group", "read", "not a member of this class")
	}

	// Get group with members
	group, err := s.repo.Group().GetByIDWithMembers(ctx, nil, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrGroupNotFound
		}
		return nil, fmt.Errorf("failed to get group with members: %w", err)
	}

	return s.buildGroupResponse(ctx, group, userID), nil
}

func (s *groupService) GetByName(ctx context.Context, name string, userID string) (*GroupResponse, error) {
	// Get group
	group, err := s.repo.Group().GetByName(ctx, nil, name)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrGroupNotFound
		}
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	// Check access permission
	canAccess, err := s.CanAccess(ctx, group.ID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, group.ID, "group", "read", "not a member of this class")
	}

	return s.buildGroupResponse(ctx, group, userID), nil
}

func (s *groupService) Update(ctx context.Context, id uint, req *UpdateGroupRequest, userID string) (*GroupResponse, error) {
	s.logger.Info("Updating group (class)", "group_id", id, "user_id", userID)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check edit permission (only owner teacher can edit)
	canEdit, err := s.CanEdit(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, NewPermissionError(userID, id, "group", "update", "only the class owner can edit")
	}

	// Get current group
	group, err := s.repo.Group().GetByID(ctx, nil, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrGroupNotFound
		}
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	// Apply updates
	if req.DisplayName != nil {
		group.DisplayName = *req.DisplayName
	}

	if req.Description != nil {
		group.Description = *req.Description
	}

	if req.Type != nil {
		group.Type = *req.Type
	}

	// Update group
	if err = s.repo.Group().Update(ctx, nil, group); err != nil {
		return nil, fmt.Errorf("failed to update group: %w", err)
	}

	s.logger.Info("Group (class) updated successfully", "group_id", id)

	return s.buildGroupResponse(ctx, group, userID), nil
}

func (s *groupService) Delete(ctx context.Context, id uint, userID string) error {
	s.logger.Info("Deleting group (class)", "group_id", id, "user_id", userID)

	// Check delete permission (only owner can delete)
	canDelete, err := s.CanDelete(ctx, id, userID)
	if err != nil {
		return err
	}
	if !canDelete {
		return NewPermissionError(userID, id, "group", "delete", "only the class owner can delete")
	}

	// Wrap in transaction to ensure atomicity with cascade deletes
	// (group_members and assessment_groups will be cascade deleted)
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Delete group (will cascade delete members and assessment assignments)
		if err := s.repo.Group().Delete(ctx, tx, id); err != nil {
			return fmt.Errorf("failed to delete group: %w", err)
		}
		return nil
	})

	if err != nil {
		s.logger.Error("Failed to delete group", "error", err, "group_id", id)
		return err
	}

	s.logger.Info("Group (class) deleted successfully", "group_id", id)

	return nil
}

// ===== LIST AND SEARCH OPERATIONS =====

func (s *groupService) List(ctx context.Context, filters repositories.GroupFilters, userID string) (*GroupListResponse, error) {
	// Check user role for filtering
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Apply membership filter based on role
	// Admin can see all groups, others only see groups they are members of
	if userRole != models.RoleAdmin {
		filters.MemberUserID = userID
	}

	// Get groups
	groups, total, err := s.repo.Group().List(ctx, nil, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}

	// Build responses
	responses := make([]*GroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, s.buildGroupResponse(ctx, group, userID))
	}

	// Calculate pagination
	page := 1
	if filters.Limit > 0 {
		page = (filters.Offset / filters.Limit) + 1
	}

	return &GroupListResponse{
		Groups: responses,
		Total:  total,
		Page:   page,
		Size:   len(responses),
	}, nil
}

func (s *groupService) GetByCreator(ctx context.Context, creatorID string, filters repositories.GroupFilters) (*GroupListResponse, error) {
	// Set creator filter
	filters.CreatedBy = creatorID

	// Get groups
	groups, err := s.repo.Group().GetByCreator(ctx, nil, creatorID)
	if err != nil {
		return nil, fmt.Errorf("failed to get groups by creator: %w", err)
	}

	// Build responses
	responses := make([]*GroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, s.buildGroupResponse(ctx, group, creatorID))
	}

	return &GroupListResponse{
		Groups: responses,
		Total:  int64(len(responses)),
		Page:   1,
		Size:   len(responses),
	}, nil
}

func (s *groupService) GetByMember(ctx context.Context, userID string, filters repositories.GroupFilters) (*GroupListResponse, error) {
	// Get user's group memberships
	memberships, err := s.repo.Group().GetMembersByUserID(ctx, nil, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user group memberships: %w", err)
	}

	// Build responses
	responses := make([]*GroupResponse, 0, len(memberships))
	for _, membership := range memberships {
		if membership.Group != nil {
			responses = append(responses, s.buildGroupResponse(ctx, membership.Group, userID))
		}
	}

	return &GroupListResponse{
		Groups: responses,
		Total:  int64(len(responses)),
		Page:   1,
		Size:   len(responses),
	}, nil
}

func (s *groupService) Search(ctx context.Context, query string, filters repositories.GroupFilters, userID string) (*GroupListResponse, error) {
	// Set search query
	filters.Query = query

	return s.List(ctx, filters, userID)
}

// ===== MEMBER MANAGEMENT =====

func (s *groupService) AddMember(ctx context.Context, groupID uint, req *AddGroupMemberRequest, userID string) error {
	s.logger.Info("Adding member to group", "group_id", groupID, "user_id", userID, "member_id", req.UserID)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check permission to manage members (only owner can add)
	canManage, err := s.CanManageMembers(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !canManage {
		return NewPermissionError(userID, groupID, "group", "add_member", "only group owner can add members")
	}

	// Verify the user being added exists
	_, err = s.repo.User().GetByID(ctx, req.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if req.Role != string(models.GroupMemberRoleMember) && req.Role != string(models.GroupMemberRoleCoOwner) {
		return ErrGroupInvalidRole
	}

	newRole := models.GroupMemberRole(req.Role)

	// All new members are added with "member" role
	// Only the creator gets "owner" role (set during group creation)
	member := &models.GroupMember{
		GroupID: groupID,
		UserID:  req.UserID,
		Role:    newRole,
	}

	if err = s.repo.Group().AddMember(ctx, nil, member); err != nil {
		if strings.Contains(err.Error(), "already a member") {
			return ErrGroupMemberExists
		}
		return fmt.Errorf("failed to add member to group: %w", err)
	}

	s.logger.Info("Member added to group successfully", "group_id", groupID, "member_id", req.UserID)

	return nil
}

func (s *groupService) RemoveMember(ctx context.Context, groupID uint, memberUserID string, userID string) error {
	s.logger.Info("Removing member from group", "group_id", groupID, "user_id", userID, "member_id", memberUserID)

	// Check permission to manage members (includes Admin bypass)
	canManage, err := s.CanManageMembers(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !canManage {
		return NewPermissionError(userID, groupID, "group", "remove_member", "only owner or co-owner can remove members")
	}

	// Check if caller is Admin (doesn't need to be member)
	isAdmin := false
	userRole, err := s.getUserRole(ctx, userID)
	if err == nil && userRole == models.RoleAdmin {
		isAdmin = true
	}

	// Get target's role
	targetRole, err := s.getMemberRole(ctx, groupID, memberUserID)
	if err != nil {
		return err
	}

	// Cannot remove owner
	if targetRole == models.GroupMemberRoleOwner {
		return ErrGroupCannotRemoveOwner
	}

	// If not Admin, check caller's role for co-owner restrictions
	if !isAdmin {
		callerRole, err := s.getMemberRole(ctx, groupID, userID)
		if err != nil {
			return err
		}
		// Co-owner cannot remove other co-owners, only owner can
		if callerRole == models.GroupMemberRoleCoOwner && targetRole == models.GroupMemberRoleCoOwner {
			return NewPermissionError(userID, groupID, "group", "remove_member", "co-owner cannot remove other co-owners")
		}
	}

	// Remove member
	if err = s.repo.Group().RemoveMember(ctx, nil, groupID, memberUserID); err != nil {
		if repositories.IsNotFoundError(err) {
			return ErrGroupMemberNotFound
		}
		return fmt.Errorf("failed to remove member from group: %w", err)
	}

	s.logger.Info("Member removed from group successfully", "group_id", groupID, "member_id", memberUserID)

	return nil
}

func (s *groupService) UpdateMemberRole(ctx context.Context, groupID uint, memberUserID string, req *UpdateMemberRoleRequest, userID string) error {
	s.logger.Info("Updating member role", "group_id", groupID, "user_id", userID, "member_id", memberUserID, "new_role", req.Role)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	newRole := models.GroupMemberRole(req.Role)

	// Validate new role is valid
	if newRole != models.GroupMemberRoleCoOwner && newRole != models.GroupMemberRoleMember {
		return fmt.Errorf("invalid role: must be 'co-owner' or 'member'")
	}

	// Check if caller is Admin (doesn't need to be member)
	isAdmin := false
	userRole, err := s.getUserRole(ctx, userID)
	if err == nil && userRole == models.RoleAdmin {
		isAdmin = true
	}

	// Get target member's current role
	targetRole, err := s.getMemberRole(ctx, groupID, memberUserID)
	if err != nil {
		return err
	}

	// Cannot change owner's role
	if targetRole == models.GroupMemberRoleOwner {
		return fmt.Errorf("cannot change owner's role")
	}

	// If not Admin, check caller's role for permission restrictions
	if !isAdmin {
		callerRole, err := s.getMemberRole(ctx, groupID, userID)
		if err != nil {
			return err
		}

		// Permission check based on caller's role
		switch callerRole {
		case models.GroupMemberRoleOwner:
			// Owner can promote/demote anyone (except owner role)

		case models.GroupMemberRoleCoOwner:
			// Co-owner can only promote member -> co-owner
			if targetRole == models.GroupMemberRoleCoOwner && newRole == models.GroupMemberRoleMember {
				return NewPermissionError(userID, groupID, "group", "update_member_role", "co-owner cannot demote other co-owners")
			}
			if newRole != models.GroupMemberRoleCoOwner {
				return NewPermissionError(userID, groupID, "group", "update_member_role", "co-owner can only promote members to co-owner")
			}

		default:
			return NewPermissionError(userID, groupID, "group", "update_member_role", "only owner or co-owner can change member roles")
		}
	}

	// Update member role
	member := &models.GroupMember{
		GroupID: groupID,
		UserID:  memberUserID,
		Role:    newRole,
	}

	if err = s.repo.Group().UpdateMember(ctx, nil, member); err != nil {
		return fmt.Errorf("failed to update member role: %w", err)
	}

	s.logger.Info("Member role updated successfully", "group_id", groupID, "member_id", memberUserID, "new_role", newRole)

	return nil
}

func (s *groupService) GetMembers(ctx context.Context, groupID uint, userID string) ([]*GroupMemberResponse, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, groupID, "group", "get_members", "not a member of this group")
	}

	// Get members
	members, err := s.repo.Group().GetMembers(ctx, nil, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group members: %w", err)
	}

	// Get caller's role for permission calculation
	callerRole, _ := s.getMemberRole(ctx, groupID, userID)

	responses := make([]*GroupMemberResponse, 0, len(members))
	for _, member := range members {
		canRemove := false
		canModify := false

		switch callerRole {
		case models.GroupMemberRoleOwner:
			// Owner can remove anyone except self, can modify anyone except owner
			if member.Role != models.GroupMemberRoleOwner {
				canRemove = true
				canModify = true
			}
		case models.GroupMemberRoleCoOwner:
			// Co-owner can remove/modify members only (not owner or other co-owners)
			if member.Role == models.GroupMemberRoleMember {
				canRemove = true
				canModify = true // Can promote to co-owner
			}
		}

		responses = append(responses, &GroupMemberResponse{
			GroupMember: member,
			CanRemove:   canRemove,
			CanModify:   canModify,
		})
	}

	return responses, nil
}

func (s *groupService) GetMemberGroups(ctx context.Context, memberUserID string) ([]*GroupResponse, error) {
	// Get user's group memberships
	memberships, err := s.repo.Group().GetMembersByUserID(ctx, nil, memberUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user group memberships: %w", err)
	}

	// Build responses
	responses := make([]*GroupResponse, 0, len(memberships))
	for _, membership := range memberships {
		if membership.Group != nil {
			responses = append(responses, s.buildGroupResponse(ctx, membership.Group, memberUserID))
		}
	}

	return responses, nil
}

// ===== PERMISSION CHECKS =====

func (s *groupService) CanAccess(ctx context.Context, groupID uint, userID string) (bool, error) {
	// Admin can access all groups
	userRole, err := s.getUserRole(ctx, userID)
	if err == nil && userRole == models.RoleAdmin {
		return true, nil
	}

	// Owner can always access
	isOwner, err := s.IsOwner(ctx, groupID, userID)
	if err != nil {
		return false, err
	}
	if isOwner {
		return true, nil
	}

	// Members can access
	return s.IsMember(ctx, groupID, userID)
}

func (s *groupService) CanEdit(ctx context.Context, groupID uint, userID string) (bool, error) {
	// Admin can edit any group
	userRole, err := s.getUserRole(ctx, userID)
	if err == nil && userRole == models.RoleAdmin {
		return true, nil
	}
	// Owner can edit their own groups
	return s.IsOwner(ctx, groupID, userID)
}

func (s *groupService) CanDelete(ctx context.Context, groupID uint, userID string) (bool, error) {
	// Admin can delete any group
	userRole, err := s.getUserRole(ctx, userID)
	if err == nil && userRole == models.RoleAdmin {
		return true, nil
	}
	// Owner can delete their own groups
	return s.IsOwner(ctx, groupID, userID)
}

func (s *groupService) CanManageMembers(ctx context.Context, groupID uint, userID string) (bool, error) {
	// Admin can manage members of any group
	userRole, err := s.getUserRole(ctx, userID)
	if err == nil && userRole == models.RoleAdmin {
		return true, nil
	}

	// Owner can manage (check via CreatedBy)
	isOwner, err := s.IsOwner(ctx, groupID, userID)
	if err != nil {
		return false, err
	}
	if isOwner {
		return true, nil
	}

	// Check if user has owner or co-owner role in group_members
	members, err := s.repo.Group().GetMembers(ctx, nil, groupID)
	if err != nil {
		return false, err
	}

	for _, member := range members {
		if member.UserID == userID {
			// Owner or co-owner can manage members
			if member.Role == models.GroupMemberRoleOwner || member.Role == models.GroupMemberRoleCoOwner {
				return true, nil
			}
		}
	}

	return false, nil
}

func (s *groupService) IsOwner(ctx context.Context, groupID uint, userID string) (bool, error) {
	group, err := s.repo.Group().GetByID(ctx, nil, groupID)
	if err != nil {
		// Translate repository error to service error for proper HTTP status mapping
		if repositories.IsNotFoundError(err) {
			return false, ErrGroupNotFound
		}
		return false, fmt.Errorf("failed to check group ownership: %w", err)
	}

	return group.CreatedBy == userID, nil
}

func (s *groupService) IsMember(ctx context.Context, groupID uint, userID string) (bool, error) {
	return s.repo.Group().IsMember(ctx, nil, groupID, userID)
}

// ===== HELPER METHODS =====

func (s *groupService) buildGroupResponse(ctx context.Context, group *models.Group, userID string) *GroupResponse {
	canEdit, _ := s.CanEdit(ctx, group.ID, userID)
	canDelete, _ := s.CanDelete(ctx, group.ID, userID)
	canManage, _ := s.CanManageMembers(ctx, group.ID, userID)
	isOwner, _ := s.IsOwner(ctx, group.ID, userID)
	isMember, _ := s.IsMember(ctx, group.ID, userID)

	// Get member role if user is a member
	var memberRole *string
	if isMember {
		members, err := s.repo.Group().GetMembers(ctx, nil, group.ID)
		if err == nil {
			for _, member := range members {
				if member.UserID == userID {
					role := string(member.Role)
					memberRole = &role
					break
				}
			}
		}
	}

	// Count members
	memberCount := len(group.Members)
	if group.Members == nil {
		members, err := s.repo.Group().GetMembers(ctx, nil, group.ID)
		if err == nil {
			memberCount = len(members)
		}
	}

	return &GroupResponse{
		Group:       group,
		CanEdit:     canEdit,
		CanDelete:   canDelete,
		CanManage:   canManage,
		MemberCount: memberCount,
		IsOwner:     isOwner,
		IsMember:    isMember,
		MemberRole:  memberRole,
	}
}

// getMemberRole returns the role of a user in a group
func (s *groupService) getMemberRole(ctx context.Context, groupID uint, userID string) (models.GroupMemberRole, error) {
	members, err := s.repo.Group().GetMembers(ctx, nil, groupID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return "", ErrGroupNotFound
		}
		return "", fmt.Errorf("failed to get members: %w", err)
	}

	for _, member := range members {
		if member.UserID == userID {
			return member.Role, nil
		}
	}

	return "", ErrGroupMemberNotFound
}

// getUserRole returns the system role of a user
func (s *groupService) getUserRole(ctx context.Context, userID string) (models.UserRole, error) {
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}
	return user.Role, nil
}
