package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openagenthub/backend/internal/auth"
	"github.com/openagenthub/backend/internal/config"
	"github.com/openagenthub/backend/internal/database"
	"github.com/openagenthub/backend/internal/middleware"
	"github.com/openagenthub/backend/internal/models"
	"github.com/openagenthub/backend/internal/response"
)

// WorkspaceHandler handles workspaces
type WorkspaceHandler struct {
	cfg *config.Config
}

func NewWorkspaceHandler(cfg *config.Config) *WorkspaceHandler {
	return &WorkspaceHandler{cfg: cfg}
}

// List lists all workspaces for the current user
func (h *WorkspaceHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var members []models.WorkspaceMember
	database.DB.Where("user_id = ? AND status = ?", userID, "active").Find(&members)
	wsIDs := make([]string, len(members))
	for i, m := range members {
		wsIDs[i] = m.WorkspaceID
	}
	var workspaces []models.Workspace
	database.DB.Where("id IN ?", wsIDs).Find(&workspaces)

	// Include role
	type wsWithRole struct {
		models.Workspace
		Role string `json:"role"`
	}
	roleMap := map[string]string{}
	for _, m := range members {
		roleMap[m.WorkspaceID] = m.Role
	}
	result := make([]wsWithRole, len(workspaces))
	for i, w := range workspaces {
		result[i] = wsWithRole{Workspace: w, Role: roleMap[w.ID]}
	}

	response.OK(c, gin.H{"items": result})
}

// Get returns workspace details
func (h *WorkspaceHandler) Get(c *gin.Context) {
	wsID := c.Param("id")
	userID := middleware.GetUserID(c)

	// Verify membership
	var member models.WorkspaceMember
	if err := database.DB.Where("user_id = ? AND workspace_id = ? AND status = ?", userID, wsID, "active").First(&member).Error; err != nil {
		response.NotFound(c, "workspace not found or access denied")
		return
	}

	var ws models.Workspace
	if err := database.DB.First(&ws, "id = ?", wsID).Error; err != nil {
		response.NotFound(c, "workspace not found")
		return
	}

	var org models.Organization
	database.DB.First(&org, "id = ?", ws.OrgID)

	response.OK(c, gin.H{
		"workspace": &ws,
		"org":       &org,
		"role":      member.Role,
	})
}

// createWorkspaceForUser creates a new workspace for the user (independent org + workspace + owner member).
// wsType is "personal" or "team", determining the workspace type.
func createWorkspaceForUser(userID, wsName, wsType string) (models.Organization, models.Workspace, error) {
	org := models.Organization{
		Name:   wsName,
		Slug:   "org-" + uuid.NewString()[:8],
		Plan:   "free",
		Status: "active",
	}
	if err := database.DB.Create(&org).Error; err != nil {
		return org, models.Workspace{}, err
	}
	ws := models.Workspace{
		OrgID:              org.ID,
		Name:               wsName,
		Slug:               "default",
		Type:               wsType,
		QuotaMemoryCount:   10000,
		QuotaToolCallDaily: 5000,
		Status:             "active",
	}
	if err := database.DB.Create(&ws).Error; err != nil {
		return org, ws, err
	}
	now := time.Now()
	member := models.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      userID,
		Role:        "owner",
		Status:      "active",
		InvitedAt:   now,
		JoinedAt:    &now,
	}
	if err := database.DB.Create(&member).Error; err != nil {
		return org, ws, err
	}
	return org, ws, nil
}

// Create creates a team (model 1: no quantity limit; after creation, the user switches into the new team)
func (h *WorkspaceHandler) Create(c *gin.Context) {
	type req struct {
		Name string `json:"name" binding:"required"`
		Slug string `json:"slug"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	userID := middleware.GetUserID(c)
	username := middleware.GetUsername(c)

	org, ws, err := createWorkspaceForUser(userID, r.Name, "team")
	if err != nil {
		response.InternalError(c, "create failed: "+err.Error())
		return
	}

	auditSvc.Log(ws.ID, userID, "user", "workspace.create", ws.ID, "workspace", nil, c.ClientIP())

	// Issue a token with the new team context so the frontend can switch directly
	var user models.User
	database.DB.First(&user, "id = ?", userID)
	token, _ := auth.GenerateJWT(h.cfg, userID, username, org.ID, ws.ID, "owner", []string{"read", "write", "admin"})

	// Load all user workspaces (including personal workspace + newly created team)
	var membersAll []models.WorkspaceMember
	database.DB.Where("user_id = ? AND status = ?", userID, "active").Find(&membersAll)
	wsIDs := make([]string, 0, len(membersAll))
	for _, m := range membersAll {
		wsIDs = append(wsIDs, m.WorkspaceID)
	}
	var allWorkspaces []models.Workspace
	database.DB.Where("id IN ?", wsIDs).Find(&allWorkspaces)

	now := time.Now()
	expireAt := now.Add(time.Duration(h.cfg.JWTExpire) * time.Hour)
	response.OK(c, loginResponse{
		Token:      token,
		ExpiresAt:  expireAt,
		User:       &user,
		Workspace:  &ws,
		Org:        &org,
		Role:       "owner",
		Workspaces: allWorkspaces,
	})
}

// Update update workspace
func (h *WorkspaceHandler) Update(c *gin.Context) {
	wsID := c.Param("id")
	userID := middleware.GetUserID(c)
	role := middleware.GetRole(c)
	if role != "owner" && role != "admin" {
		response.Forbidden(c, "admin role required")
		return
	}

	type req struct {
		Name               *string `json:"name"`
		QuotaMemoryCount   *int    `json:"quota_memory_count"`
		QuotaToolCallDaily *int    `json:"quota_tool_call_daily"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	updates := map[string]interface{}{}
	if r.Name != nil {
		updates["name"] = *r.Name
	}
	if r.QuotaMemoryCount != nil {
		updates["quota_memory_count"] = *r.QuotaMemoryCount
	}
	if r.QuotaToolCallDaily != nil {
		updates["quota_tool_call_daily"] = *r.QuotaToolCallDaily
	}
	if len(updates) == 0 {
		response.BadRequest(c, "no fields to update")
		return
	}

	if err := database.DB.Model(&models.Workspace{}).Where("id = ?", wsID).Updates(updates).Error; err != nil {
		response.InternalError(c, "update failed: "+err.Error())
		return
	}

	auditSvc.Log(wsID, userID, "user", "workspace.update", wsID, "workspace", updates, c.ClientIP())
	response.OK(c, gin.H{"updated": true})
}

// Delete delete workspace
func (h *WorkspaceHandler) Delete(c *gin.Context) {
	wsID := c.Param("id")
	role := middleware.GetRole(c)
	if role != "owner" {
		response.Forbidden(c, "owner role required")
		return
	}

	// Personal workspace cannot be deleted
	var ws models.Workspace
	if err := database.DB.First(&ws, "id = ?", wsID).Error; err != nil {
		response.NotFound(c, "workspace not found")
		return
	}
	if ws.Type == "personal" {
		response.BadRequest(c, "personal workspace cannot be deleted")
		return
	}

	if err := database.DB.Delete(&models.Workspace{}, "id = ?", wsID).Error; err != nil {
		response.InternalError(c, "delete failed: "+err.Error())
		return
	}
	// Cascade delete members
	database.DB.Where("workspace_id = ?", wsID).Delete(&models.WorkspaceMember{})

	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "workspace.delete", wsID, "workspace", nil, c.ClientIP())
	response.OK(c, gin.H{"deleted": true})
}

// Members member management
type MemberHandler struct{}

func NewMemberHandler() *MemberHandler {
	return &MemberHandler{}
}

// ListMembers list members
func (h *MemberHandler) ListMembers(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	var members []models.WorkspaceMember
	database.DB.Where("workspace_id = ?", wsID).Find(&members)

	userIDs := make([]string, len(members))
	for i, m := range members {
		userIDs[i] = m.UserID
	}
	var users []models.User
	database.DB.Where("id IN ?", userIDs).Find(&users)
	userMap := map[string]models.User{}
	for _, u := range users {
		userMap[u.ID] = u
	}

	type userBrief struct {
		ID          string `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		AvatarURL   string `json:"avatar_url"`
	}
	type memberWithUser struct {
		models.WorkspaceMember
		User *userBrief `json:"user"`
	}
	result := make([]memberWithUser, len(members))
	for i, m := range members {
		u := userMap[m.UserID]
		result[i] = memberWithUser{
			WorkspaceMember: m,
			User: &userBrief{
				ID:          u.ID,
				Username:    u.Username,
				DisplayName: u.DisplayName,
				AvatarURL:   u.AvatarURL,
			},
		}
	}
	response.OK(c, gin.H{"items": result})
}

// InviteMember invite member
func (h *MemberHandler) InviteMember(c *gin.Context) {
	role := middleware.GetRole(c)
	if role != "owner" && role != "admin" {
		response.Forbidden(c, "admin role required")
		return
	}
	type req struct {
		Username string `json:"username" binding:"required"`
		Role     string `json:"role" binding:"required,oneof=admin member viewer"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	// Validate username format
	if err := auth.ValidateUsername(r.Username); err != nil {
		response.BadRequest(c, "invalid username: "+err.Error())
		return
	}
	wsID := middleware.GetWorkspaceID(c)
	inviterID := middleware.GetUserID(c)

	var user models.User
	if err := database.DB.Where("username = ?", r.Username).First(&user).Error; err != nil {
		response.NotFound(c, "user not found; please register first")
		return
	}
	// Existing member? Differentiate by status
	var existing models.WorkspaceMember
	if err := database.DB.Where("user_id = ? AND workspace_id = ?", user.ID, wsID).First(&existing).Error; err == nil {
		if existing.Status == "active" {
			response.Fail(c, 409, 40900, "already a workspace member")
			return
		}
		if existing.Status == "pending" {
			response.Fail(c, 409, 40900, "already invited, waiting for acceptance")
			return
		}
	}
	now := time.Now()
	member := models.WorkspaceMember{
		WorkspaceID: wsID,
		UserID:      user.ID,
		Role:        r.Role,
		Status:      "pending",
		InvitedBy:   inviterID,
		InvitedAt:   now,
		JoinedAt:    nil,
	}
	if err := database.DB.Create(&member).Error; err != nil {
		response.InternalError(c, "create failed: "+err.Error())
		return
	}

	auditSvc.Log(wsID, inviterID, "user", "member.invite", user.ID, "user", gin.H{"role": r.Role}, c.ClientIP())
	response.OK(c, member)
}

// UpdateMemberRole update member role
func (h *MemberHandler) UpdateMemberRole(c *gin.Context) {
	callerRole := middleware.GetRole(c)
	if callerRole != "owner" && callerRole != "admin" {
		response.Forbidden(c, "admin role required")
		return
	}
	wsID := middleware.GetWorkspaceID(c)
	memberID := c.Param("id")

	// Search only within the current workspace to prevent cross-workspace privilege escalation
	var member models.WorkspaceMember
	if err := database.DB.First(&member, "id = ? AND workspace_id = ?", memberID, wsID).Error; err != nil {
		response.NotFound(c, "member not found")
		return
	}
	if member.Status != "active" {
		response.BadRequest(c, "cannot update role of a pending member")
		return
	}
	type req struct {
		Role string `json:"role" binding:"required,oneof=owner admin member viewer"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	// Permission guardrails:
	// 1) owner role cannot be changed (ownership transfer requires a separate process to prevent admin downgrade)
	if member.Role == "owner" {
		response.Forbidden(c, "cannot change the owner's role")
		return
	}
	// 2) The owner role cannot be assigned via this endpoint (to prevent creating a second owner / privilege escalation)
	if r.Role == "owner" {
		response.Forbidden(c, "cannot assign the owner role here")
		return
	}
	// 3) Only the owner can manage admins (admins cannot promote/demote other admins)
	if callerRole != "owner" && (member.Role == "admin" || r.Role == "admin") {
		response.Forbidden(c, "only the owner can manage admins")
		return
	}

	database.DB.Model(&models.WorkspaceMember{}).Where("id = ?", memberID).Update("role", r.Role)
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "member.update_role", memberID, "member", gin.H{"role": r.Role}, c.ClientIP())
	response.OK(c, gin.H{"updated": true})
}

// RemoveMember removes a workspace member
func (h *MemberHandler) RemoveMember(c *gin.Context) {
	callerRole := middleware.GetRole(c)
	if callerRole != "owner" && callerRole != "admin" {
		response.Forbidden(c, "admin role required")
		return
	}
	wsID := middleware.GetWorkspaceID(c)
	memberID := c.Param("id")

	var member models.WorkspaceMember
	if err := database.DB.First(&member, "id = ? AND workspace_id = ?", memberID, wsID).Error; err != nil {
		response.NotFound(c, "member not found")
		return
	}
	// Owner cannot be removed (owner leaving = deleting the team)
	if member.Role == "owner" {
		response.Forbidden(c, "cannot remove the owner")
		return
	}
	// Only the owner can remove an admin
	if callerRole != "owner" && member.Role == "admin" {
		response.Forbidden(c, "only the owner can remove an admin")
		return
	}

	database.DB.Delete(&models.WorkspaceMember{}, "id = ? AND workspace_id = ?", memberID, wsID)
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "member.remove", memberID, "member", nil, c.ClientIP())
	response.OK(c, gin.H{"deleted": true})
}

// ListMyInvitations lists pending invitations for the current user (across workspaces)
func (h *MemberHandler) ListMyInvitations(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var members []models.WorkspaceMember
	database.DB.Where("user_id = ? AND status = ?", userID, "pending").Find(&members)

	wsIDs := make([]string, len(members))
	for i, m := range members {
		wsIDs[i] = m.WorkspaceID
	}
	var workspaces []models.Workspace
	database.DB.Where("id IN ?", wsIDs).Find(&workspaces)
	wsMap := map[string]models.Workspace{}
	for _, w := range workspaces {
		wsMap[w.ID] = w
	}

	type invitationItem struct {
		models.WorkspaceMember
		Workspace *models.Workspace `json:"workspace"`
	}
	result := make([]invitationItem, len(members))
	for i, m := range members {
		w := wsMap[m.WorkspaceID]
		result[i] = invitationItem{
			WorkspaceMember: m,
			Workspace:       &w,
		}
	}
	response.OK(c, gin.H{"items": result})
}

// AcceptInvitation accept invitation
func (h *MemberHandler) AcceptInvitation(c *gin.Context) {
	userID := middleware.GetUserID(c)
	invitationID := c.Param("id")

	var member models.WorkspaceMember
	if err := database.DB.Where("id = ? AND user_id = ?", invitationID, userID).First(&member).Error; err != nil {
		response.NotFound(c, "invitation not found")
		return
	}
	if member.Status != "pending" {
		response.BadRequest(c, "invitation is not pending")
		return
	}

	now := time.Now()
	database.DB.Model(&member).Updates(map[string]interface{}{
		"status":    "active",
		"joined_at": &now,
	})

	auditSvc.Log(member.WorkspaceID, userID, "user", "member.accept_invite", member.ID, "member", nil, c.ClientIP())
	response.OK(c, gin.H{"accepted": true})
}

// RejectInvitation reject invitation (delete record)
func (h *MemberHandler) RejectInvitation(c *gin.Context) {
	userID := middleware.GetUserID(c)
	invitationID := c.Param("id")

	var member models.WorkspaceMember
	if err := database.DB.Where("id = ? AND user_id = ?", invitationID, userID).First(&member).Error; err != nil {
		response.NotFound(c, "invitation not found")
		return
	}
	if member.Status != "pending" {
		response.BadRequest(c, "invitation is not pending")
		return
	}

	database.DB.Delete(&member)

	auditSvc.Log(member.WorkspaceID, userID, "user", "member.reject_invite", member.ID, "member", nil, c.ClientIP())
	response.OK(c, gin.H{"rejected": true})
}

// LeaveWorkspace removes the current user from a team workspace they have joined.
// The owner cannot leave (they should delete the team or transfer ownership). After leaving, the user retains their personal workspace
// and can switch back to it via the workspace switcher.
func (h *MemberHandler) LeaveWorkspace(c *gin.Context) {
	userID := middleware.GetUserID(c)
	wsID := c.Param("id")

	var member models.WorkspaceMember
	if err := database.DB.Where("user_id = ? AND workspace_id = ? AND status = ?", userID, wsID, "active").First(&member).Error; err != nil {
		response.NotFound(c, "membership not found")
		return
	}
	if member.Role == "owner" {
		response.BadRequest(c, "owner cannot leave; delete or transfer the team instead")
		return
	}
	if err := database.DB.Delete(&member).Error; err != nil {
		response.InternalError(c, "leave failed: "+err.Error())
		return
	}

	auditSvc.Log(wsID, userID, "user", "member.leave", member.ID, "member", nil, c.ClientIP())
	response.OK(c, gin.H{"left": true})
}
