package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openagenthub/backend/internal/auth"
	"github.com/openagenthub/backend/internal/config"
	"github.com/openagenthub/backend/internal/database"
	"github.com/openagenthub/backend/internal/middleware"
	"github.com/openagenthub/backend/internal/models"
	"github.com/openagenthub/backend/internal/response"
)

// AuthHandler handles authentication
type AuthHandler struct {
	cfg *config.Config
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

type loginRequest struct {
	Username    string `json:"username" form:"username" binding:"required"`
	Password    string `json:"password" form:"password" binding:"required,min=6"`
	WorkspaceID string `json:"workspace_id" form:"workspace_id"`
}

type loginResponse struct {
	Token      string               `json:"token"`
	ExpiresAt  time.Time            `json:"expires_at"`
	User       *models.User         `json:"user"`
	Workspace  *models.Workspace    `json:"workspace"`
	Org        *models.Organization `json:"org"`
	Role       string               `json:"role"`
	Workspaces []models.Workspace   `json:"workspaces"`
}

// displayNameOf returns the user's display name, falling back to username if empty.
func displayNameOf(u models.User) string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Username
}

// Login handles user login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	// Look up user
	var user models.User
	if err := database.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		response.Unauthorized(c, "invalid username or password")
		return
	}

	if !auth.CheckPasswordHash(req.Password, user.PasswordHash) {
		response.Unauthorized(c, "invalid username or password")
		return
	}

	if user.Status != "active" {
		response.Forbidden(c, "user is disabled")
		return
	}

	// Query workspaces the user belongs to
	var members []models.WorkspaceMember
	if err := database.DB.Where("user_id = ? AND status = ?", user.ID, "active").Find(&members).Error; err != nil {
		response.InternalError(c, "failed to query workspaces: "+err.Error())
		return
	}

	if len(members) == 0 {
		now := time.Now()
		database.DB.Model(&user).Update("last_login_at", &now)
		expireAt := now.Add(time.Duration(h.cfg.JWTExpire) * time.Hour)

		// No workspace yet — auto-create a personal workspace so the user is ready to go.
		// Pending invitations are shown in the Dashboard for the user to accept.
		org, ws, err := createWorkspaceForUser(user.ID, displayNameOf(user)+"'s Personal Workspace", "personal")
		if err != nil {
			response.InternalError(c, "failed to create personal workspace: "+err.Error())
			return
		}
		token, err := auth.GenerateJWT(h.cfg, user.ID, user.Username, org.ID, ws.ID, "owner", []string{"read", "write", "admin"})
		if err != nil {
			response.InternalError(c, "failed to generate token: "+err.Error())
			return
		}
		response.OK(c, loginResponse{
			Token: token, ExpiresAt: expireAt, User: &user,
			Workspace: &ws, Org: &org, Role: "owner", Workspaces: []models.Workspace{ws},
		})
		return
	}

	// Select workspace: prefer personal workspace as default
	selectedWSID := req.WorkspaceID
	selectedMember := members[0]
	for _, m := range members {
		var w models.Workspace
		if err := database.DB.First(&w, "id = ?", m.WorkspaceID).Error; err == nil && w.Type == "personal" {
			selectedMember = m
			break
		}
	}
	if selectedWSID != "" {
		found := false
		for _, m := range members {
			if m.WorkspaceID == selectedWSID {
				selectedMember = m
				found = true
				break
			}
		}
		if !found {
			response.Forbidden(c, "access to the specified workspace denied")
			return
		}
	}

	// Load workspace
	var ws models.Workspace
	if err := database.DB.First(&ws, "id = ?", selectedMember.WorkspaceID).Error; err != nil {
		response.InternalError(c, "failed to load workspace")
		return
	}

	// Load org
	var org models.Organization
	if err := database.DB.First(&org, "id = ?", ws.OrgID).Error; err != nil {
		response.InternalError(c, "failed to load organization")
		return
	}

	// Load all workspaces
	wsIDs := make([]string, 0, len(members))
	for _, m := range members {
		wsIDs = append(wsIDs, m.WorkspaceID)
	}
	var workspaces []models.Workspace
	database.DB.Where("id IN ?", wsIDs).Find(&workspaces)

	// Generate JWT
	scopes := []string{"read", "write"}
	if selectedMember.Role == "owner" || selectedMember.Role == "admin" {
		scopes = append(scopes, "admin")
	}
	token, err := auth.GenerateJWT(h.cfg, user.ID, user.Username, org.ID, ws.ID, selectedMember.Role, scopes)
	if err != nil {
		response.InternalError(c, "failed to generate token: "+err.Error())
		return
	}

	// Update last login time
	now := time.Now()
	database.DB.Model(&user).Update("last_login_at", &now)

	// Audit
	auditSvc.LogSync(ws.ID, user.ID, "user", "user.login", user.ID, "user", nil, c.ClientIP())

	expireAt := now.Add(time.Duration(h.cfg.JWTExpire) * time.Hour)
	response.OK(c, loginResponse{
		Token:      token,
		ExpiresAt:  expireAt,
		User:       &user,
		Workspace:  &ws,
		Org:        &org,
		Role:       selectedMember.Role,
		Workspaces: workspaces,
	})
}

// Register handles user registration (dev environment only)
func (h *AuthHandler) Register(c *gin.Context) {
	type req struct {
		Username    string `json:"username" form:"username" binding:"required"`
		Password    string `json:"password" form:"password" binding:"required,min=6"`
		DisplayName string `json:"display_name" form:"display_name"`
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

	// Check username uniqueness
	var existing models.User
	if err := database.DB.Where("username = ?", r.Username).First(&existing).Error; err == nil {
		response.Fail(c, 409, 40900, "username already registered")
		return
	}

	hash, _ := auth.HashPassword(r.Password)
	displayName := r.DisplayName
	if displayName == "" {
		displayName = r.Username
	}
	user := models.User{
		Username:     r.Username,
		PasswordHash: hash,
		DisplayName:  displayName,
		Status:       "active",
	}
	if err := database.DB.Create(&user).Error; err != nil {
		response.InternalError(c, "failed to create user: "+err.Error())
		return
	}

	// Registration auto-creates a personal workspace (named after display name),
	// so the user is ready to go. They can later create team workspaces or accept invitations.
	org, ws, err := createWorkspaceForUser(user.ID, displayName+"'s Personal Workspace", "personal")
	if err != nil {
		response.InternalError(c, "failed to create personal workspace: "+err.Error())
		return
	}

	now := time.Now()
	token, _ := auth.GenerateJWT(h.cfg, user.ID, user.Username, org.ID, ws.ID, "owner", []string{"read", "write", "admin"})

	expireAt := now.Add(time.Duration(h.cfg.JWTExpire) * time.Hour)
	response.OK(c, loginResponse{
		Token:      token,
		ExpiresAt:  expireAt,
		User:       &user,
		Workspace:  &ws,
		Org:        &org,
		Role:       "owner",
		Workspaces: []models.Workspace{ws},
	})
}

// Me returns the current user info
func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)
	wsID := middleware.GetWorkspaceID(c)

	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		response.NotFound(c, "user not found")
		return
	}

	var ws models.Workspace
	if err := database.DB.First(&ws, "id = ?", wsID).Error; err != nil {
		response.NotFound(c, "workspace not found")
		return
	}

	var org models.Organization
	database.DB.First(&org, "id = ?", ws.OrgID)

	// All workspaces for the user
	var members []models.WorkspaceMember
	database.DB.Where("user_id = ? AND status = ?", userID, "active").Find(&members)
	wsIDs := make([]string, len(members))
	for i, m := range members {
		wsIDs[i] = m.WorkspaceID
	}
	var workspaces []models.Workspace
	database.DB.Where("id IN ?", wsIDs).Find(&workspaces)

	role := middleware.GetRole(c)
	response.OK(c, gin.H{
		"user":       &user,
		"workspace":  &ws,
		"org":        &org,
		"role":       role,
		"workspaces": workspaces,
	})
}

// SwitchWorkspace switches the active workspace
func (h *AuthHandler) SwitchWorkspace(c *gin.Context) {
	type req struct {
		WorkspaceID string `json:"workspace_id" binding:"required"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	userID := middleware.GetUserID(c)
	var member models.WorkspaceMember
	if err := database.DB.Where("user_id = ? AND workspace_id = ? AND status = ?", userID, r.WorkspaceID, "active").First(&member).Error; err != nil {
		response.Forbidden(c, "access to this workspace denied")
		return
	}

	var user models.User
	database.DB.First(&user, "id = ?", userID)

	var ws models.Workspace
	database.DB.First(&ws, "id = ?", r.WorkspaceID)

	var org models.Organization
	database.DB.First(&org, "id = ?", ws.OrgID)

	scopes := []string{"read", "write"}
	if member.Role == "owner" || member.Role == "admin" {
		scopes = append(scopes, "admin")
	}
	token, _ := auth.GenerateJWT(h.cfg, user.ID, user.Username, org.ID, ws.ID, member.Role, scopes)

	now := time.Now()
	expireAt := now.Add(time.Duration(h.cfg.JWTExpire) * time.Hour)

	// Load all workspaces
	var membersAll []models.WorkspaceMember
	database.DB.Where("user_id = ? AND status = ?", userID, "active").Find(&membersAll)
	wsIDs := make([]string, len(membersAll))
	for i, m := range membersAll {
		wsIDs[i] = m.WorkspaceID
	}
	var workspaces []models.Workspace
	database.DB.Where("id IN ?", wsIDs).Find(&workspaces)

	response.OK(c, loginResponse{
		Token:      token,
		ExpiresAt:  expireAt,
		User:       &user,
		Workspace:  &ws,
		Org:        &org,
		Role:       member.Role,
		Workspaces: workspaces,
	})
}
