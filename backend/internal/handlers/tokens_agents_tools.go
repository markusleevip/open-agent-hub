package handlers

import (
	"crypto/rand"
	"encoding/base64"
	stdjson "encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openagenthub/backend/internal/auth"
	"github.com/openagenthub/backend/internal/config"
	"github.com/openagenthub/backend/internal/database"
	"github.com/openagenthub/backend/internal/middleware"
	"github.com/openagenthub/backend/internal/models"
	"github.com/openagenthub/backend/internal/response"
	"golang.org/x/crypto/bcrypt"
)

// TokenHandler handles MCP Tokens
type TokenHandler struct {
	cfg *config.Config
}

func NewTokenHandler(cfg *config.Config) *TokenHandler {
	return &TokenHandler{cfg: cfg}
}

// List lists tokens
func (h *TokenHandler) List(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	var keys []models.APIKey
	database.DB.Where("workspace_id = ?", wsID).Order("created_at DESC").Find(&keys)

	type item struct {
		models.APIKey
		TokenPreview string `json:"token_preview"`
	}
	items := make([]item, len(keys))
	for i, k := range keys {
		items[i] = item{APIKey: k, TokenPreview: k.Prefix + "****"}
	}
	response.OK(c, gin.H{"items": items})
}

// Create creates a token (generates plaintext token and returns it once)
func (h *TokenHandler) Create(c *gin.Context) {
	type req struct {
		Name          string     `json:"name" binding:"required"`
		Scopes        []string   `json:"scopes"`
		ExpiresInDays *int       `json:"expires_in_days"`
		Expires       *time.Time `json:"expires_at"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	wsID := middleware.GetWorkspaceID(c)
	userID := middleware.GetUserID(c)

	// Generate token
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	token := "pat_" + base64.RawURLEncoding.EncodeToString(tokenBytes)
	prefix := token[:11]
	hash, _ := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)

	scopes := r.Scopes
	if len(scopes) == 0 {
		scopes = []string{"read", "write"}
	}
	scopesJSON, _ := jsonMarshal(scopes)

	var expiresAt *time.Time
	if r.Expires != nil {
		expiresAt = r.Expires
	} else if r.ExpiresInDays != nil && *r.ExpiresInDays > 0 {
		t := time.Now().AddDate(0, 0, *r.ExpiresInDays)
		expiresAt = &t
	}

	key := models.APIKey{
		WorkspaceID: wsID,
		Name:        r.Name,
		Prefix:      prefix,
		Hash:        string(hash),
		Scopes:      scopesJSON,
		ExpiresAt:   expiresAt,
		CreatedBy:   userID,
	}
	if err := database.DB.Create(&key).Error; err != nil {
		response.InternalError(c, "create failed: "+err.Error())
		return
	}

	auditSvc.Log(wsID, userID, "user", "token.create", key.ID, "api_key", gin.H{"name": r.Name}, c.ClientIP())
	// Return plaintext once
	response.OK(c, gin.H{
		"api_key": &key,
		"token":   token, // only time shown
		"warning": "store this token securely; it will not be shown in full again",
	})
}

// Revoke revokes a token
func (h *TokenHandler) Revoke(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")
	now := time.Now()
	if err := database.DB.Model(&models.APIKey{}).Where("id = ? AND workspace_id = ?", id, wsID).Update("revoked_at", &now).Error; err != nil {
		response.InternalError(c, "revoke failed: "+err.Error())
		return
	}
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "token.revoke", id, "api_key", nil, c.ClientIP())
	response.OK(c, gin.H{"revoked": true})
}

// Delete permanently deletes a token
func (h *TokenHandler) Delete(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).Delete(&models.APIKey{}).Error; err != nil {
		response.InternalError(c, "delete failed: "+err.Error())
		return
	}
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "token.delete", id, "api_key", nil, c.ClientIP())
	response.OK(c, gin.H{"deleted": true})
}

func jsonMarshal(v interface{}) (string, error) {
	b, err := stdjson.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// AgentClientHandler handles Agent Clients
type AgentClientHandler struct{}

func NewAgentClientHandler() *AgentClientHandler {
	return &AgentClientHandler{}
}

// List lists tokens
func (h *AgentClientHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var clients []models.AgentClient
	// Agent Profile is user-private data, decoupled from workspace: returns all agent clients
	// for this user by user_id, does not change with current workspace, and is not shared
	// among team members.
	database.DB.Where("user_id = ?", userID).
		Order("last_seen_at DESC NULLS LAST, first_seen_at DESC").Find(&clients)
	response.OK(c, gin.H{"items": clients})
}

// Get returns agent client details
func (h *AgentClientHandler) Get(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id := c.Param("id")
	var ac models.AgentClient
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&ac).Error; err != nil {
		response.NotFound(c, "agent client not found")
		return
	}

	// Associated sessions
	var sessions []models.MCPSession
	database.DB.Where("agent_client_id = ?", id).Order("started_at DESC").Limit(50).Find(&sessions)

	// Call statistics
	var totalCalls int64
	database.DB.Model(&models.ToolInvocationLog{}).Where("agent_client_id = ?", id).Count(&totalCalls)

	response.OK(c, gin.H{
		"client":      &ac,
		"sessions":    sessions,
		"total_calls": totalCalls,
	})
}

// Delete removes an agent client
func (h *AgentClientHandler) Delete(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	userID := middleware.GetUserID(c)
	id := c.Param("id")
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&models.AgentClient{}).Error; err != nil {
		response.InternalError(c, "delete failed: "+err.Error())
		return
	}
	auditSvc.Log(wsID, userID, "user", "agent_client.delete", id, "agent_client", nil, c.ClientIP())
	response.OK(c, gin.H{"deleted": true})
}

// ConnectedServerHandler handles Connected MCP Servers
type ConnectedServerHandler struct{}

func NewConnectedServerHandler() *ConnectedServerHandler {
	return &ConnectedServerHandler{}
}

// List lists tokens
func (h *ConnectedServerHandler) List(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	var servers []models.ConnectedMCPServer
	database.DB.Where("workspace_id = ?", wsID).Order("created_at DESC").Find(&servers)
	response.OK(c, gin.H{"items": servers})
}

// Get returns connected server details
func (h *ConnectedServerHandler) Get(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")
	var s models.ConnectedMCPServer
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).First(&s).Error; err != nil {
		response.NotFound(c, "MCP server not found")
		return
	}
	// Associated tool policies
	var policies []models.ToolPolicy
	database.DB.Where("connected_server_id = ?", id).Find(&policies)
	response.OK(c, gin.H{
		"server":        &s,
		"tool_policies": policies,
	})
}

// redactedMark replaces sensitive credentials with a redacted placeholder
// (empty values return empty, to distinguish "not set").
func redactedMark(s string) string {
	if s == "" {
		return ""
	}
	return "***redacted***"
}

// Create registers a new external MCP Server
func (h *ConnectedServerHandler) Create(c *gin.Context) {
	type req struct {
		Name        string `json:"name" binding:"required"`
		DisplayName string `json:"display_name"`
		Endpoint    string `json:"endpoint" binding:"required,url"`
		Transport   string `json:"transport"`
		AuthType    string `json:"auth_type"`
		AuthConfig  string `json:"auth_config"`
		ToolsJSON   string `json:"tools_json"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	wsID := middleware.GetWorkspaceID(c)
	if r.Transport == "" {
		r.Transport = "streamable_http"
	}
	if r.AuthType == "" {
		r.AuthType = "none"
	}
	if r.ToolsJSON == "" {
		r.ToolsJSON = "[]"
	}

	s := models.ConnectedMCPServer{
		WorkspaceID: wsID,
		Name:        r.Name,
		DisplayName: r.DisplayName,
		Endpoint:    r.Endpoint,
		Transport:   r.Transport,
		AuthType:    r.AuthType,
		AuthConfig:  database.EncryptAES(r.AuthConfig), // encrypt credentials at rest
		ToolsJSON:   r.ToolsJSON,
		PolicyJSON:  "{}",
		Status:      "pending",
	}
	if err := database.DB.Create(&s).Error; err != nil {
		response.InternalError(c, "create failed: "+err.Error())
		return
	}
	r.AuthConfig = redactedMark(r.AuthConfig) // redact for audit log; avoid plaintext credentials in audit_logs
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "mcp_server.create", s.ID, "mcp_server", r, c.ClientIP())
	response.OK(c, s)
}

// Update updates a connected MCP server
func (h *ConnectedServerHandler) Update(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")
	var s models.ConnectedMCPServer
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).First(&s).Error; err != nil {
		response.NotFound(c, "MCP server not found")
		return
	}
	type req struct {
		DisplayName *string `json:"display_name"`
		Endpoint    *string `json:"endpoint"`
		AuthConfig  *string `json:"auth_config"`
		ToolsJSON   *string `json:"tools_json"`
		Status      *string `json:"status"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	updates := map[string]interface{}{}
	if r.DisplayName != nil {
		updates["display_name"] = *r.DisplayName
	}
	if r.Endpoint != nil {
		updates["endpoint"] = *r.Endpoint
	}
	if r.AuthConfig != nil {
		updates["auth_config"] = database.EncryptAES(*r.AuthConfig) // encrypt credentials at rest
	}
	if r.ToolsJSON != nil {
		updates["tools_json"] = *r.ToolsJSON
	}
	if r.Status != nil {
		updates["status"] = *r.Status
	}
	if len(updates) == 0 {
		response.BadRequest(c, "no fields to update")
		return
	}
	database.DB.Model(&s).Updates(updates)
	if _, ok := updates["auth_config"]; ok {
		updates["auth_config"] = redactedMark(*r.AuthConfig) // redact for audit
	}
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "mcp_server.update", id, "mcp_server", updates, c.ClientIP())
	database.DB.First(&s, "id = ?", id)
	response.OK(c, s)
}

// Delete
func (h *ConnectedServerHandler) Delete(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")
	database.DB.Where("id = ? AND workspace_id = ?", id, wsID).Delete(&models.ConnectedMCPServer{})
	database.DB.Where("connected_server_id = ?", id).Delete(&models.ToolPolicy{})
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "mcp_server.delete", id, "mcp_server", nil, c.ClientIP())
	response.OK(c, gin.H{"deleted": true})
}

// ToolPolicyHandler handles tool policies
type ToolPolicyHandler struct{}

func NewToolPolicyHandler() *ToolPolicyHandler {
	return &ToolPolicyHandler{}
}

// List lists tokens
func (h *ToolPolicyHandler) List(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	var policies []models.ToolPolicy
	database.DB.Where("workspace_id = ?", wsID).Order("tool_name").Find(&policies)
	response.OK(c, gin.H{"items": policies})
}

// Update updates a tool policy
func (h *ToolPolicyHandler) Update(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")
	var p models.ToolPolicy
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).First(&p).Error; err != nil {
		response.NotFound(c, "policy not found")
		return
	}
	type req struct {
		Allowed              *bool   `json:"allowed"`
		RequiresConfirmation *bool   `json:"requires_confirmation"`
		MaxCallsPerDay       *int    `json:"max_calls_per_day"`
		MaxCallsPerUser      *int    `json:"max_calls_per_user"`
		RiskLevel            *string `json:"risk_level"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	updates := map[string]interface{}{}
	if r.Allowed != nil {
		updates["allowed"] = *r.Allowed
	}
	if r.RequiresConfirmation != nil {
		updates["requires_confirmation"] = *r.RequiresConfirmation
	}
	if r.MaxCallsPerDay != nil {
		updates["max_calls_per_day"] = *r.MaxCallsPerDay
	}
	if r.MaxCallsPerUser != nil {
		updates["max_calls_per_user"] = *r.MaxCallsPerUser
	}
	if r.RiskLevel != nil {
		updates["risk_level"] = *r.RiskLevel
	}
	if len(updates) == 0 {
		response.BadRequest(c, "no fields to update")
		return
	}
	database.DB.Model(&p).Updates(updates)
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "tool_policy.update", id, "tool_policy", updates, c.ClientIP())
	database.DB.First(&p, "id = ?", id)
	response.OK(c, p)
}

// ToolInvocationLogHandler handles tool invocation logs
type ToolInvocationLogHandler struct{}

func NewToolInvocationLogHandler() *ToolInvocationLogHandler {
	return &ToolInvocationLogHandler{}
}

// List lists tokens
func (h *ToolInvocationLogHandler) List(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	toolName := c.Query("tool_name")
	status := c.Query("status")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	q := database.DB.Where("workspace_id = ?", wsID)
	if toolName != "" {
		q = q.Where("tool_name = ?", toolName)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}

	var total int64
	q.Model(&models.ToolInvocationLog{}).Count(&total)

	var logs []models.ToolInvocationLog
	q.Order("invoked_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)

	response.OK(c, gin.H{
		"items":     logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// UsageHandler handles usage statistics
type UsageHandler struct{}

func NewUsageHandler() *UsageHandler {
	return &UsageHandler{}
}

// Dashboard returns dashboard data
func (h *UsageHandler) Dashboard(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	today := time.Now().Format("2006-01-02")
	thisMonth := time.Now().Format("2006-01")

	// Today's calls
	var todayCalls int64
	database.DB.Model(&models.UsageRecord{}).Where("workspace_id = ? AND period = ? AND metric = ?", wsID, today, "tool_call").Select("COALESCE(SUM(quantity), 0)").Scan(&todayCalls)
	// This month's calls
	var monthCalls int64
	database.DB.Model(&models.UsageRecord{}).Where("workspace_id = ? AND period = ? AND metric = ?", wsID, thisMonth, "tool_call").Select("COALESCE(SUM(quantity), 0)").Scan(&monthCalls)
	// Active sessions
	var activeSessions int64
	database.DB.Model(&models.MCPSession{}).Where("workspace_id = ? AND status = ?", wsID, "active").Count(&activeSessions)
	// Memory writes
	var memoryWrites int64
	database.DB.Model(&models.UsageRecord{}).Where("workspace_id = ? AND period = ? AND metric = ?", wsID, thisMonth, "memory_write").Select("COALESCE(SUM(quantity), 0)").Scan(&memoryWrites)
	// Total memories
	var memoryTotal int64
	database.DB.Model(&models.Memory{}).Where("workspace_id = ? AND state = 'active'", wsID).Count(&memoryTotal)

	// Trend: last 7 days
	type dayPoint struct {
		Date  string `json:"date"`
		Count int    `json:"count"`
	}
	trend := make([]dayPoint, 0, 7)
	for i := 6; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		var c int64
		database.DB.Model(&models.UsageRecord{}).Where("workspace_id = ? AND period = ? AND metric = ?", wsID, d, "tool_call").Select("COALESCE(SUM(quantity), 0)").Scan(&c)
		trend = append(trend, dayPoint{Date: d, Count: int(c)})
	}

	// Top 5 by tool
	type toolPoint struct {
		Tool  string `json:"tool"`
		Count int    `json:"count"`
	}
	var topTools []toolPoint
	database.DB.Model(&models.ToolInvocationLog{}).
		Select("tool_name as tool, COUNT(*) as count").
		Where("workspace_id = ?", wsID).
		Group("tool_name").
		Order("count DESC").
		Limit(5).Scan(&topTools)

	response.OK(c, gin.H{
		"today_calls":     todayCalls,
		"month_calls":     monthCalls,
		"active_sessions": activeSessions,
		"memory_writes":   memoryWrites,
		"memory_total":    memoryTotal,
		"trend_7d":        trend,
		"top_tools":       topTools,
	})
}

// AuditHandler handles audit logs
type AuditHandler struct{}

func NewAuditHandler() *AuditHandler {
	return &AuditHandler{}
}

// List lists audit logs
func (h *AuditHandler) List(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	action := c.Query("action")
	actor := c.Query("actor")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	q := database.DB.Where("workspace_id = ?", wsID)
	if action != "" {
		q = q.Where("action = ?", action)
	}
	if actor != "" {
		q = q.Where("actor = ?", actor)
	}
	var total int64
	q.Model(&models.AuditLog{}).Count(&total)
	var logs []models.AuditLog
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)
	response.OK(c, gin.H{
		"items":     logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// auth reference kept (to prevent unused import from being stripped)
var _ = auth.GenerateJWT
