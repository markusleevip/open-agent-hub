package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openagenthub/backend/internal/database"
	"github.com/openagenthub/backend/internal/middleware"
	"github.com/openagenthub/backend/internal/models"
	"github.com/openagenthub/backend/internal/response"
	"github.com/openagenthub/backend/internal/services"
)

// RuleHandler handles rules
type RuleHandler struct{}

func NewRuleHandler() *RuleHandler {
	return &RuleHandler{}
}

// List lists rules
func (h *RuleHandler) List(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	scope := c.Query("scope")
	ruleType := c.Query("type")
	search := c.Query("q")

	q := database.DB.Where("workspace_id = ?", wsID)
	if scope != "" {
		q = q.Where("scope = ?", scope)
	}
	if ruleType != "" {
		q = q.Where("type = ?", ruleType)
	}
	if search != "" {
		q = q.Where("name LIKE ? OR value LIKE ? OR description LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	var total int64
	q.Model(&models.Rule{}).Count(&total)

	var rules []models.Rule
	q.Order("updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rules)

	response.OK(c, gin.H{
		"items":     rules,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Get returns rule details
func (h *RuleHandler) Get(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")
	var rule models.Rule
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).First(&rule).Error; err != nil {
		response.NotFound(c, "rule not found")
		return
	}
	response.OK(c, rule)
}

// Create creates a rule
func (h *RuleHandler) Create(c *gin.Context) {
	type req struct {
		Name        string  `json:"name" binding:"required"`
		Description string  `json:"description"`
		Value       string  `json:"value" binding:"required"`
		Type        string  `json:"type" binding:"required"`
		Tags        string  `json:"tags"`
		Scope       string  `json:"scope"`
		ProjectID   *string `json:"project_id"`
		AgentName   *string `json:"agent_name"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if r.Scope == "" {
		r.Scope = "workspace"
	}
	if r.Tags == "" {
		r.Tags = "[]"
	}
	orgID := middleware.GetOrgID(c)
	wsID := middleware.GetWorkspaceID(c)
	userID := middleware.GetUserID(c)

	rule := models.Rule{
		OrgID:       orgID,
		WorkspaceID: wsID,
		ProjectID:   r.ProjectID,
		AgentName:   r.AgentName,
		Name:        r.Name,
		Description: r.Description,
		Value:       r.Value,
		Type:        r.Type,
		Tags:        r.Tags,
		Scope:       r.Scope,
		Version:     1,
	}
	if err := database.DB.Create(&rule).Error; err != nil {
		response.InternalError(c, "create failed: "+err.Error())
		return
	}
	auditSvc.Log(wsID, userID, "user", "rule.create", rule.ID, "rule", r, c.ClientIP())
	response.OK(c, rule)
}

// Update updates a rule
func (h *RuleHandler) Update(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")
	var rule models.Rule
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).First(&rule).Error; err != nil {
		response.NotFound(c, "rule not found")
		return
	}

	type req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Value       *string `json:"value"`
		Type        *string `json:"type"`
		Tags        *string `json:"tags"`
		Scope       *string `json:"scope"`
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
	if r.Description != nil {
		updates["description"] = *r.Description
	}
	if r.Value != nil {
		updates["value"] = *r.Value
	}
	if r.Type != nil {
		updates["type"] = *r.Type
	}
	if r.Tags != nil {
		updates["tags"] = *r.Tags
	}
	if r.Scope != nil {
		updates["scope"] = *r.Scope
	}
	updates["version"] = rule.Version + 1
	if len(updates) == 1 { // only version
		response.BadRequest(c, "no fields to update")
		return
	}
	database.DB.Model(&rule).Updates(updates)
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "rule.update", id, "rule", updates, c.ClientIP())

	// Reload
	database.DB.First(&rule, "id = ?", id)
	response.OK(c, rule)
}

// Delete deletes a rule
func (h *RuleHandler) Delete(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).Delete(&models.Rule{}).Error; err != nil {
		response.InternalError(c, "delete failed: "+err.Error())
		return
	}
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "rule.delete", id, "rule", nil, c.ClientIP())
	response.OK(c, gin.H{"deleted": true})
}

// GetGlobalRules returns the effective global rules for the current workspace
// (resolved by target agent override, with ETag)
func (h *RuleHandler) GetGlobalRules(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	agentName := optStr(c.Query("agent_name"))

	// Workspace-level resolution: project dimension is nil
	effective := services.ResolveEffectiveRules(wsID, nil, agentName)
	etag := services.RuleETag(effective)

	if match := c.GetHeader("If-None-Match"); match != "" && match == etag {
		c.Status(304)
		return
	}

	c.Header("ETag", etag)
	c.Header("Cache-Control", "max-age=300, must-revalidate")
	c.Header("Last-Modified", time.Now().UTC().Format(httpDateFormat))

	response.OK(c, gin.H{
		"rules":   services.RuleValues(effective),
		"detail":  effective,
		"version": time.Now().UTC().Format(time.RFC3339),
		"etag":    etag,
	})
}

// optStr converts a query parameter to *string: empty string -> nil
func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

const httpDateFormat = "Mon, 02 Jan 2006 15:04:05 GMT"

// GetProjectRules returns effective project-level rules (global->project->agent override merge)
func (h *RuleHandler) GetProjectRules(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	projectID := c.Query("project_id")
	if projectID == "" {
		response.BadRequest(c, "project_id is required")
		return
	}
	agentName := optStr(c.Query("agent_name"))

	effective := services.ResolveEffectiveRules(wsID, &projectID, agentName)

	// Raw layers (for debug/display only)
	var projectRules []models.Rule
	database.DB.Where("workspace_id = ? AND project_id = ?", wsID, projectID).Order("type, name").Find(&projectRules)
	var globalRules []models.Rule
	database.DB.Where("workspace_id = ? AND project_id IS NULL", wsID).Order("type, name").Find(&globalRules)

	etag := services.RuleETag(effective)
	if match := c.GetHeader("If-None-Match"); match != "" && match == etag {
		c.Status(304)
		return
	}
	c.Header("ETag", etag)
	c.Header("Cache-Control", "max-age=300, must-revalidate")
	response.OK(c, gin.H{
		"rules":           services.RuleValues(effective),
		"effective_rules": services.RuleValues(effective),
		"effective":       effective,
		"project_rules":   projectRules,
		"global_rules":    globalRules,
	})
}

// GetWorkspacePolicy returns the workspace policy
func (h *RuleHandler) GetWorkspacePolicy(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	var ws models.Workspace
	database.DB.First(&ws, "id = ?", wsID)

	var policies []models.ToolPolicy
	database.DB.Where("workspace_id = ?", wsID).Find(&policies)

	// Usage statistics
	today := time.Now().Format("2006-01-02")
	var todayCount int64
	database.DB.Model(&models.UsageRecord{}).Where("workspace_id = ? AND period = ?", wsID, today).Select("COALESCE(SUM(quantity), 0)").Scan(&todayCount)

	response.OK(c, gin.H{
		"quotas": gin.H{
			"memory_count_max":    ws.QuotaMemoryCount,
			"tool_call_daily_max": ws.QuotaToolCallDaily,
		},
		"tool_policies": policies,
		"today_usage":   todayCount,
	})
}

// GetOutputPreferences returns user output preferences
func (h *RuleHandler) GetOutputPreferences(c *gin.Context) {
	userID := middleware.GetUserID(c)
	prefsMap, err := services.GetOutputPreferencesMap(userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, prefsMap)
}

// GetPersonalInstructions returns personalized instructions for the current user
func (h *RuleHandler) GetPersonalInstructions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	instructions, err := services.GetPersonalInstructions(userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, instructions)
}

// UpdatePersonalInstructions update the current user personal instructions
func (h *RuleHandler) UpdatePersonalInstructions(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	userID := middleware.GetUserID(c)

	var req services.PersonalInstructions
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	saved, changed, err := services.SavePersonalInstructions(userID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	payload := gin.H{
		"changed_keys": changed,
	}
	if strings.TrimSpace(saved.CustomInstructions) != "" {
		sum := sha256.Sum256([]byte(saved.CustomInstructions))
		payload["custom_instructions_length"] = len([]rune(saved.CustomInstructions))
		payload["custom_instructions_sha256"] = hex.EncodeToString(sum[:])
	}
	auditSvc.Log(wsID, userID, "user", "personal_instructions.update", userID, "output_preference", payload, c.ClientIP())

	response.OK(c, saved)
}
