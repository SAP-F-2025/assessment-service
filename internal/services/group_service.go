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
	s.logger.Info("Creating group (class)", "creator_id", creatorID, "name", req.Name)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// IMPORTANT: Only teachers can create groups (classes)
	creator, err := s.repo.User().GetByID(ctx, creatorID)
	if err != nil {
		return nil, fmt.Errorf("failed to get creator user: %w", err)
	}

	if creator.Role != models.RoleTeacher && creator.Role != models.RoleAdmin {
		return nil, NewPermissionError(creatorID, 0, "group", "create", "only teachers can create groups (classes)")
	}

	// Check if group with same name exists
	exists, err := s.repo.Group().ExistsByName(ctx, nil, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check group name uniqueness: %w", err)
	}
	if exists {
		return nil, ErrGroupDuplicateName
	}

	// Create group
	group := &models.Group{
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

	if err = s.repo.Group().Create(ctx, nil, group); err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	s.logger.Info("Group (class) created successfully", "group_id", group.ID, "creator", creatorID)

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
	// Get groups
	groups, _, err := s.repo.Group().List(ctx, nil, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}

	// Build responses - only include groups user can access
	responses := make([]*GroupResponse, 0, len(groups))
	for _, group := range groups {
		// Check if user can access this group
		canAccess, err := s.CanAccess(ctx, group.ID, userID)
		if err != nil || !canAccess {
			continue
		}
		responses = append(responses, s.buildGroupResponse(ctx, group, userID))
	}

	// Calculate pagination
	page := (filters.Offset / filters.Limit) + 1
	if filters.Limit == 0 {
		page = 1
	}

	return &GroupListResponse{
		Groups: responses,
		Total:  int64(len(responses)), // Adjusted total based on access
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
	s.logger.Info("Adding member to group (class)", "group_id", groupID, "user_id", userID, "member_id", req.UserID)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check permission to manage members (only owner or teacher members can add)
	canManage, err := s.CanManageMembers(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !canManage {
		return NewPermissionError(userID, groupID, "group", "add_member", "only class owner or teachers can add members")
	}

	// Get the user being added to determine their role
	user, err := s.repo.User().GetByID(ctx, req.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Determine member role
	var memberRole models.GroupMemberRole
	if req.Role != "" {
		// Use provided role but validate it matches user's system role
		memberRole = models.GroupMemberRole(req.Role)

		// Validate: teachers can only be added as teacher role, students as student role
		if user.Role == models.RoleTeacher && memberRole != models.GroupMemberRoleTeacher {
			return fmt.Errorf("teachers must be added with 'teacher' role")
		}
		if user.Role == models.RoleStudent && memberRole != models.GroupMemberRoleStudent {
			return fmt.Errorf("students must be added with 'student' role")
		}
	} else {
		// Auto-detect role from user's system role
		if user.Role == models.RoleTeacher || user.Role == models.RoleAdmin {
			memberRole = models.GroupMemberRoleTeacher
		} else {
			memberRole = models.GroupMemberRoleStudent
		}
	}

	// Create member
	member := &models.GroupMember{
		GroupID: groupID,
		UserID:  req.UserID,
		Role:    memberRole,
	}

	if err = s.repo.Group().AddMember(ctx, nil, member); err != nil {
		if strings.Contains(err.Error(), "already a member") {
			return ErrGroupMemberExists
		}
		return fmt.Errorf("failed to add member to group: %w", err)
	}

	s.logger.Info("Member added to group (class) successfully", "group_id", groupID, "member_id", req.UserID, "role", memberRole)

	return nil
}

func (s *groupService) RemoveMember(ctx context.Context, groupID uint, memberUserID string, userID string) error {
	s.logger.Info("Removing member from group (class)", "group_id", groupID, "user_id", userID, "member_id", memberUserID)

	// Check permission to manage members
	canManage, err := s.CanManageMembers(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !canManage {
		return NewPermissionError(userID, groupID, "group", "remove_member", "only class owner or teachers can remove members")
	}

	// Check if trying to remove the owner
	isOwner, err := s.IsOwner(ctx, groupID, memberUserID)
	if err != nil {
		return err
	}
	if isOwner {
		return ErrGroupCannotRemoveOwner
	}

	// Teachers can only remove students, not other teachers (unless they are the owner)
	isCallerOwner, err := s.IsOwner(ctx, groupID, userID)
	if err != nil {
		return err
	}

	if !isCallerOwner {
		// Non-owner teachers can only remove students
		members, err := s.repo.Group().GetMembers(ctx, nil, groupID)
		if err != nil {
			return fmt.Errorf("failed to get members: %w", err)
		}

		for _, member := range members {
			if member.UserID == memberUserID {
				if member.Role == models.GroupMemberRoleTeacher {
					return NewPermissionError(userID, groupID, "group", "remove_member", "only class owner can remove teachers")
				}
				break
			}
		}
	}

	// Remove member
	if err = s.repo.Group().RemoveMember(ctx, nil, groupID, memberUserID); err != nil {
		if repositories.IsNotFoundError(err) {
			return ErrGroupMemberNotFound
		}
		return fmt.Errorf("failed to remove member from group: %w", err)
	}

	s.logger.Info("Member removed from group (class) successfully", "group_id", groupID, "member_id", memberUserID)

	return nil
}

func (s *groupService) UpdateMemberRole(ctx context.Context, groupID uint, memberUserID string, req *UpdateMemberRoleRequest, userID string) error {
	s.logger.Info("Updating member role", "group_id", groupID, "user_id", userID, "member_id", memberUserID)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Only owner can update member roles
	isOwner, err := s.IsOwner(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !isOwner {
		return NewPermissionError(userID, groupID, "group", "update_member_role", "only class owner can change member roles")
	}

	// Check if trying to change owner's role (owner is not a member)
	isTargetOwner, err := s.IsOwner(ctx, groupID, memberUserID)
	if err != nil {
		return err
	}
	if isTargetOwner {
		return fmt.Errorf("cannot change class owner's role")
	}

	// Get the user to validate role matches their system role
	user, err := s.repo.User().GetByID(ctx, memberUserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	newRole := models.GroupMemberRole(req.Role)

	// Validate role matches user's system role
	if user.Role == models.RoleTeacher && newRole != models.GroupMemberRoleTeacher {
		return fmt.Errorf("teachers must have 'teacher' role in class")
	}
	if user.Role == models.RoleStudent && newRole != models.GroupMemberRoleStudent {
		return fmt.Errorf("students must have 'student' role in class")
	}

	// Create member object with new role
	member := &models.GroupMember{
		GroupID: groupID,
		UserID:  memberUserID,
		Role:    newRole,
	}

	// Update member role directly (preserves joined_at and other metadata)
	if err = s.repo.Group().UpdateMember(ctx, nil, member); err != nil {
		return fmt.Errorf("failed to update member role: %w", err)
	}

	s.logger.Info("Member role updated successfully", "group_id", groupID, "member_id", memberUserID, "role", req.Role)

	return nil
}

func (s *groupService) GetMembers(ctx context.Context, groupID uint, userID string) ([]*GroupMemberResponse, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, groupID, "group", "get_members", "not a member of this class")
	}

	// Get members
	members, err := s.repo.Group().GetMembers(ctx, nil, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group members: %w", err)
	}

	// Build responses
	canManage, _ := s.CanManageMembers(ctx, groupID, userID)
	isOwner, _ := s.IsOwner(ctx, groupID, userID)

	responses := make([]*GroupMemberResponse, 0, len(members))
	for _, member := range members {
		canRemove := false
		canModify := false

		if isOwner {
			// Owner can remove and modify anyone
			canRemove = true
			canModify = true
		} else if canManage && member.Role == models.GroupMemberRoleStudent {
			// Teachers can only remove/modify students
			canRemove = true
			canModify = false // Only owner can modify roles
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
	// Owner can always access
	isOwner, err := s.IsOwner(ctx, groupID, userID)
	if err != nil {
		return false, err
	}
	if isOwner {
		return true, nil
	}

	// Members can access
	isMember, err := s.IsMember(ctx, groupID, userID)
	if err != nil {
		return false, err
	}

	return isMember, nil
}

func (s *groupService) CanEdit(ctx context.Context, groupID uint, userID string) (bool, error) {
	// Only owner (teacher who created the class) can edit
	return s.IsOwner(ctx, groupID, userID)
}

func (s *groupService) CanDelete(ctx context.Context, groupID uint, userID string) (bool, error) {
	// Only owner can delete
	return s.IsOwner(ctx, groupID, userID)
}

func (s *groupService) CanManageMembers(ctx context.Context, groupID uint, userID string) (bool, error) {
	// Owner can manage
	isOwner, err := s.IsOwner(ctx, groupID, userID)
	if err != nil {
		return false, err
	}
	if isOwner {
		return true, nil
	}

	// Teacher members can also add/remove students
	members, err := s.repo.Group().GetMembers(ctx, nil, groupID)
	if err != nil {
		return false, err
	}

	for _, member := range members {
		if member.UserID == userID && member.Role == models.GroupMemberRoleTeacher {
			return true, nil
		}
	}

	return false, nil
}

func (s *groupService) IsOwner(ctx context.Context, groupID uint, userID string) (bool, error) {
	group, err := s.repo.Group().GetByID(ctx, nil, groupID)
	if err != nil {
		return false, err
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
