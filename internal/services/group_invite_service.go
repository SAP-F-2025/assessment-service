package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ===== INVITE REQUEST/RESPONSE TYPES =====

// CreateInviteLinkRequest is the request for creating an invite link
type CreateInviteLinkRequest struct {
	MaxUses     *int       `json:"max_uses,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	DefaultRole string     `json:"default_role,omitempty" validate:"omitempty,oneof=member co-owner"`
}

// CreateInviteCodeRequest is the request for creating an invite code
type CreateInviteCodeRequest struct {
	MaxUses     *int       `json:"max_uses,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	DefaultRole string     `json:"default_role,omitempty" validate:"omitempty,oneof=member co-owner"`
}

// JoinViaCodeRequest is the request for joining via invite code
type JoinViaCodeRequest struct {
	Code string `json:"code" validate:"required,min=6,max=8"`
}

// GroupInviteResponse is the response for a group invite
type GroupInviteResponse struct {
	*models.GroupInvite
	InviteURL     string `json:"invite_url,omitempty"`
	IsExpired     bool   `json:"is_expired"`
	IsExhausted   bool   `json:"is_exhausted"`
	CanUse        bool   `json:"can_use"`
	RemainingUses int    `json:"remaining_uses"` // -1 means unlimited
}

// ===== INVITE MANAGEMENT METHODS =====

// CreateInviteLink creates an invite link for a group
func (s *groupService) CreateInviteLink(ctx context.Context, groupID uint, req *CreateInviteLinkRequest, userID string) (*GroupInviteResponse, error) {
	s.logger.Info("Creating invite link", "group_id", groupID, "user_id", userID)

	// Check permission (owner or co-owner can create invites)
	canManage, err := s.CanManageMembers(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, NewPermissionError(userID, groupID, "group", "create_invite", "only owner or co-owner can create invites")
	}

	// Verify group exists
	_, err = s.repo.Group().GetByID(ctx, nil, groupID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrGroupNotFound
		}
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	// Generate unique token (retry if collision - extremely unlikely with UUID+random)
	var token string
	for i := 0; i < 10; i++ {
		token = generateInviteToken()
		exists, err := s.repo.GroupInvite().ExistsByToken(ctx, nil, token)
		if err != nil {
			return nil, fmt.Errorf("failed to check token uniqueness: %w", err)
		}
		if !exists {
			break
		}
		if i == 9 {
			return nil, fmt.Errorf("failed to generate unique token after 10 attempts")
		}
	}

	// Set default expiration (7 days)
	var expiresAt *time.Time
	if req != nil && req.ExpiresAt != nil {
		expiresAt = req.ExpiresAt
	} else {
		defaultExpiry := time.Now().Add(time.Duration(models.DefaultInviteExpirationDays) * 24 * time.Hour)
		expiresAt = &defaultExpiry
	}

	// Set default role
	defaultRole := models.GroupMemberRoleMember
	if req != nil && req.DefaultRole != "" {
		defaultRole = models.GroupMemberRole(req.DefaultRole)
	}

	// Create invite
	invite := &models.GroupInvite{
		GroupID:     groupID,
		Token:       token,
		Type:        models.InviteTypeLink,
		MaxUses:     nil, // Unlimited by default
		ExpiresAt:   expiresAt,
		DefaultRole: defaultRole,
		CreatedBy:   userID,
	}

	if req != nil && req.MaxUses != nil {
		invite.MaxUses = req.MaxUses
	}

	if err := s.repo.GroupInvite().Create(ctx, nil, invite); err != nil {
		return nil, fmt.Errorf("failed to create invite link: %w", err)
	}

	s.logger.Info("Invite link created successfully", "group_id", groupID, "invite_id", invite.ID)

	return s.buildInviteResponse(invite), nil
}

// CreateInviteCode creates an invite code for a group
func (s *groupService) CreateInviteCode(ctx context.Context, groupID uint, req *CreateInviteCodeRequest, userID string) (*GroupInviteResponse, error) {
	s.logger.Info("Creating invite code", "group_id", groupID, "user_id", userID)

	// Check permission
	canManage, err := s.CanManageMembers(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, NewPermissionError(userID, groupID, "group", "create_invite", "only owner or co-owner can create invites")
	}

	// Verify group exists
	_, err = s.repo.Group().GetByID(ctx, nil, groupID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrGroupNotFound
		}
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	// Generate unique code (retry if collision)
	var code string
	codeFound := false
	for i := 0; i < 10; i++ {
		code = generateInviteCode()
		exists, err := s.repo.GroupInvite().ExistsByCode(ctx, nil, code)
		if err != nil {
			return nil, fmt.Errorf("failed to check code uniqueness: %w", err)
		}
		if !exists {
			codeFound = true
			break
		}
	}
	if !codeFound {
		return nil, fmt.Errorf("failed to generate unique code after 10 attempts, please try again")
	}

	// Generate unique token (retry if collision)
	var token string
	for i := 0; i < 10; i++ {
		token = generateInviteToken()
		exists, err := s.repo.GroupInvite().ExistsByToken(ctx, nil, token)
		if err != nil {
			return nil, fmt.Errorf("failed to check token uniqueness: %w", err)
		}
		if !exists {
			break
		}
		if i == 9 {
			return nil, fmt.Errorf("failed to generate unique token after 10 attempts")
		}
	}

	// Set default expiration (7 days)
	var expiresAt *time.Time
	if req != nil && req.ExpiresAt != nil {
		expiresAt = req.ExpiresAt
	} else {
		defaultExpiry := time.Now().Add(time.Duration(models.DefaultInviteExpirationDays) * 24 * time.Hour)
		expiresAt = &defaultExpiry
	}

	// Set default role
	defaultRole := models.GroupMemberRoleMember
	if req != nil && req.DefaultRole != "" {
		defaultRole = models.GroupMemberRole(req.DefaultRole)
	}

	// Create invite
	invite := &models.GroupInvite{
		GroupID:     groupID,
		Token:       token,
		Code:        &code,
		Type:        models.InviteTypeCode,
		MaxUses:     nil, // Unlimited by default
		ExpiresAt:   expiresAt,
		DefaultRole: defaultRole,
		CreatedBy:   userID,
	}

	if req != nil && req.MaxUses != nil {
		invite.MaxUses = req.MaxUses
	}

	if err := s.repo.GroupInvite().Create(ctx, nil, invite); err != nil {
		return nil, fmt.Errorf("failed to create invite code: %w", err)
	}

	s.logger.Info("Invite code created successfully", "group_id", groupID, "invite_id", invite.ID, "code", code)

	return s.buildInviteResponse(invite), nil
}

// GetGroupInvites returns all invites for a group
func (s *groupService) GetGroupInvites(ctx context.Context, groupID uint, userID string) ([]*GroupInviteResponse, error) {
	// Check permission
	canManage, err := s.CanManageMembers(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, NewPermissionError(userID, groupID, "group", "view_invites", "only owner or co-owner can view invites")
	}

	invites, err := s.repo.GroupInvite().GetByGroupID(ctx, nil, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group invites: %w", err)
	}

	responses := make([]*GroupInviteResponse, len(invites))
	for i, invite := range invites {
		responses[i] = s.buildInviteResponse(invite)
	}

	return responses, nil
}

// DeleteInvite deletes an invite
func (s *groupService) DeleteInvite(ctx context.Context, inviteID uint, userID string) error {
	s.logger.Info("Deleting invite", "invite_id", inviteID, "user_id", userID)

	// Get invite to check group ownership
	invite, err := s.repo.GroupInvite().GetByID(ctx, nil, inviteID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return ErrGroupInviteNotFound
		}
		return fmt.Errorf("failed to get invite: %w", err)
	}

	// Check permission
	canManage, err := s.CanManageMembers(ctx, invite.GroupID, userID)
	if err != nil {
		return err
	}
	if !canManage {
		return NewPermissionError(userID, invite.GroupID, "group", "delete_invite", "only owner or co-owner can delete invites")
	}

	if err := s.repo.GroupInvite().Delete(ctx, nil, inviteID); err != nil {
		return fmt.Errorf("failed to delete invite: %w", err)
	}

	s.logger.Info("Invite deleted successfully", "invite_id", inviteID)

	return nil
}

// RegenerateInvite regenerates the token/code for an invite
func (s *groupService) RegenerateInvite(ctx context.Context, inviteID uint, userID string) (*GroupInviteResponse, error) {
	s.logger.Info("Regenerating invite", "invite_id", inviteID, "user_id", userID)

	// Get invite
	invite, err := s.repo.GroupInvite().GetByID(ctx, nil, inviteID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrGroupInviteNotFound
		}
		return nil, fmt.Errorf("failed to get invite: %w", err)
	}

	// Check permission
	canManage, err := s.CanManageMembers(ctx, invite.GroupID, userID)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, NewPermissionError(userID, invite.GroupID, "group", "regenerate_invite", "only owner or co-owner can regenerate invites")
	}

	// Generate new token
	invite.Token = generateInviteToken()

	// If it's a code type, also regenerate the code
	if invite.Type == models.InviteTypeCode {
		newCode := generateInviteCode()
		invite.Code = &newCode
	}

	// Reset uses count
	invite.UsesCount = 0

	// Reset expiration to 7 days from now
	newExpiry := time.Now().Add(time.Duration(models.DefaultInviteExpirationDays) * 24 * time.Hour)
	invite.ExpiresAt = &newExpiry

	if err := s.repo.GroupInvite().Update(ctx, nil, invite); err != nil {
		return nil, fmt.Errorf("failed to regenerate invite: %w", err)
	}

	s.logger.Info("Invite regenerated successfully", "invite_id", inviteID)

	return s.buildInviteResponse(invite), nil
}

// ===== JOIN VIA INVITE METHODS =====

// JoinViaLink allows a user to join a group via an invite link token
func (s *groupService) JoinViaLink(ctx context.Context, token string, userID string) (*GroupResponse, error) {
	s.logger.Info("User joining group via link", "user_id", userID)

	// Get invite by token
	invite, err := s.repo.GroupInvite().GetByToken(ctx, nil, token)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrGroupInviteNotFound
		}
		return nil, fmt.Errorf("failed to get invite: %w", err)
	}

	return s.joinViaInvite(ctx, invite, userID)
}

// JoinViaCode allows a user to join a group via an invite code
func (s *groupService) JoinViaCode(ctx context.Context, code string, userID string) (*GroupResponse, error) {
	s.logger.Info("User joining group via code", "user_id", userID, "code", code)

	// Normalize code to uppercase
	code = strings.ToUpper(strings.TrimSpace(code))

	// Get invite by code
	invite, err := s.repo.GroupInvite().GetByCode(ctx, nil, code)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrGroupInviteNotFound
		}
		return nil, fmt.Errorf("failed to get invite: %w", err)
	}

	return s.joinViaInvite(ctx, invite, userID)
}

// joinViaInvite is the common logic for joining via invite (link or code)
func (s *groupService) joinViaInvite(ctx context.Context, invite *models.GroupInvite, userID string) (*GroupResponse, error) {
	group, err := s.repo.Group().GetByID(ctx, nil, invite.GroupID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			// Group has been deleted - return invite not found to avoid info disclosure
			s.logger.Warn("Attempt to join deleted group via invite",
				"group_id", invite.GroupID,
				"invite_id", invite.ID,
				"user_id", userID)
			return nil, ErrGroupInviteNotFound
		}
		return nil, fmt.Errorf("failed to verify group: %w", err)
	}

	// Check if invite is valid
	if !invite.CanUse() {
		if invite.IsExpired() {
			return nil, ErrGroupInviteExpired
		}
		if invite.IsExhausted() {
			return nil, ErrGroupInviteExhausted
		}
		return nil, ErrGroupInviteInvalid
	}

	// Check if user is already a member
	isMember, err := s.IsMember(ctx, invite.GroupID, userID)
	if err != nil {
		return nil, err
	}
	if isMember {
		return nil, ErrGroupMemberExists
	}

	// Verify user exists
	_, err = s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify user: %w", err)
	}

	// Add user as member with the invite's default role
	member := &models.GroupMember{
		GroupID: invite.GroupID,
		UserID:  userID,
		Role:    invite.DefaultRole,
	}

	// Use transaction to add member and increment uses count
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.Group().AddMember(ctx, tx, member); err != nil {
			return fmt.Errorf("failed to add member: %w", err)
		}

		if err := s.repo.GroupInvite().IncrementUsesCount(ctx, tx, invite.ID); err != nil {
			return fmt.Errorf("failed to increment uses count: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("User joined group via invite", "group_id", invite.GroupID, "user_id", userID, "invite_id", invite.ID)

	return s.buildGroupResponse(ctx, group, userID), nil
}

// ===== LEAVE GROUP METHOD =====

// LeaveGroup allows a member to leave a group
func (s *groupService) LeaveGroup(ctx context.Context, groupID uint, userID string) error {
	s.logger.Info("User leaving group", "group_id", groupID, "user_id", userID)

	// Check if user is a member
	isMember, err := s.IsMember(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrGroupMemberNotFound
	}

	// Get member role - owner cannot leave their own group
	memberRole, err := s.getMemberRole(ctx, groupID, userID)
	if err != nil {
		return err
	}

	if memberRole == models.GroupMemberRoleOwner {
		return ErrGroupOwnerCannotLeave
	}

	// Remove the member
	if err := s.repo.Group().RemoveMember(ctx, nil, groupID, userID); err != nil {
		return fmt.Errorf("failed to leave group: %w", err)
	}

	s.logger.Info("User left group successfully", "group_id", groupID, "user_id", userID)

	return nil
}

// ===== HELPER FUNCTIONS =====

// buildInviteResponse builds a GroupInviteResponse from a GroupInvite model
func (s *groupService) buildInviteResponse(invite *models.GroupInvite) *GroupInviteResponse {
	response := &GroupInviteResponse{
		GroupInvite:   invite,
		IsExpired:     invite.IsExpired(),
		IsExhausted:   invite.IsExhausted(),
		CanUse:        invite.CanUse(),
		RemainingUses: invite.RemainingUses(),
	}

	// Generate invite URL for link type
	if invite.Type == models.InviteTypeLink {
		// The actual base URL should be configured from environment
		// For now, use a placeholder that frontend can construct
		response.InviteURL = fmt.Sprintf("/join/%s", invite.Token)
	}

	return response
}

// generateInviteToken generates a secure random token for invite links
func generateInviteToken() string {
	// Use UUID v4 + additional random bytes for extra security
	uuidPart := uuid.New().String()
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	return strings.ReplaceAll(uuidPart, "-", "") + hex.EncodeToString(randomBytes)
}

// generateInviteCode generates a short, human-readable invite code
func generateInviteCode() string {
	// Use only uppercase letters and numbers, excluding ambiguous characters (0, O, I, L, 1)
	const charset = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	const codeLength = 6

	code := make([]byte, codeLength)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		code[i] = charset[n.Int64()]
	}

	return string(code)
}
