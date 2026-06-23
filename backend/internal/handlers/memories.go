package handlers

import (
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openagenthub/backend/internal/database"
	"github.com/openagenthub/backend/internal/middleware"
	"github.com/openagenthub/backend/internal/models"
	"github.com/openagenthub/backend/internal/response"
	"github.com/openagenthub/backend/internal/services"
)

// MemoryHandler handles memories
type MemoryHandler struct{}

func NewMemoryHandler() *MemoryHandler {
	return &MemoryHandler{}
}

type memoryListItem struct {
	models.Memory
	Relevance float64 `json:"relevance,omitempty"`
}

// List lists memories
func (h *MemoryHandler) List(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	memType := c.Query("type")
	category := c.Query("category")
	provenance := c.Query("provenance")
	scope := c.Query("scope")
	search := c.Query("q")
	pinned := c.Query("pinned")
	state := c.DefaultQuery("state", "active")

	q := database.DB.Where("workspace_id = ?", wsID)
	if memType != "" {
		q = q.Where("type = ?", memType)
	}
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if provenance != "" {
		q = q.Where("provenance = ?", provenance)
	}
	if scope != "" {
		q = q.Where("scope = ?", scope)
	}
	if state != "" {
		q = q.Where("state = ?", state)
	}
	if pinned == "true" {
		q = q.Where("pinned = ?", true)
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
	q.Model(&models.Memory{}).Count(&total)

	var memories []models.Memory
	q.Order("pinned DESC, importance DESC, updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&memories)

	// If a search term is provided, compute similarity
	if search != "" {
		items := make([]memoryListItem, len(memories))
		for i, m := range memories {
			score := services.Relevance(search, m.Content)
			score += m.Importance * 0.3 // importance weighting
			if m.Pinned {
				score += 0.2
			}
			items[i] = memoryListItem{Memory: m, Relevance: math.Min(score, 1.0)}
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].Relevance > items[j].Relevance
		})
		response.OK(c, gin.H{
			"items":     items,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		})
		return
	}

	response.OK(c, gin.H{
		"items":     memories,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Get returns memory details
func (h *MemoryHandler) Get(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")
	var m models.Memory
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).First(&m).Error; err != nil {
		response.NotFound(c, "memory not found")
		return
	}
	response.OK(c, m)
}

// Create creates a memory
func (h *MemoryHandler) Create(c *gin.Context) {
	type req struct {
		Content    string  `json:"content" binding:"required"`
		Type       string  `json:"type" binding:"required"`
		Category   string  `json:"category" binding:"required"`
		Tags       string  `json:"tags"`
		Scope      string  `json:"scope"`
		Provenance string  `json:"provenance"`
		Importance float64 `json:"importance"`
		Pinned     bool    `json:"pinned"`
		ProjectID  *string `json:"project_id"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if r.Scope == "" {
		r.Scope = "workspace"
	}
	if r.Provenance == "" {
		r.Provenance = "human_curated"
	}
	if r.Importance == 0 {
		r.Importance = 0.5
	}
	if r.Tags == "" {
		r.Tags = "[]"
	}
	orgID := middleware.GetOrgID(c)
	wsID := middleware.GetWorkspaceID(c)
	userID := middleware.GetUserID(c)

	m := models.Memory{
		OrgID:       orgID,
		WorkspaceID: wsID,
		ProjectID:   r.ProjectID,
		UserID:      userID,
		Content:     r.Content,
		Type:        r.Type,
		Category:    r.Category,
		Tags:        r.Tags,
		Scope:       r.Scope,
		Provenance:  r.Provenance,
		Importance:  r.Importance,
		Pinned:      r.Pinned,
		State:       "active",
		CharCount:   len([]rune(r.Content)),
	}
	if err := database.DB.Create(&m).Error; err != nil {
		response.InternalError(c, "create failed: "+err.Error())
		return
	}
	auditSvc.Log(wsID, userID, "user", "memory.create", m.ID, "memory", r, c.ClientIP())
	usageSvc.Record(wsID, userID, "memory_write", 1)
	response.OK(c, m)
}

// Update updates a memory
func (h *MemoryHandler) Update(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")
	var m models.Memory
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).First(&m).Error; err != nil {
		response.NotFound(c, "memory not found")
		return
	}
	type req struct {
		Content    *string  `json:"content"`
		Tags       *string  `json:"tags"`
		Importance *float64 `json:"importance"`
		Pinned     *bool    `json:"pinned"`
		State      *string  `json:"state"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	updates := map[string]interface{}{}
	if r.Content != nil {
		updates["content"] = *r.Content
		updates["char_count"] = len([]rune(*r.Content))
	}
	if r.Tags != nil {
		updates["tags"] = *r.Tags
	}
	if r.Importance != nil {
		updates["importance"] = *r.Importance
	}
	if r.Pinned != nil {
		updates["pinned"] = *r.Pinned
	}
	if r.State != nil && m.Type == "skill" {
		updates["state"] = *r.State
	}
	if len(updates) == 0 {
		response.BadRequest(c, "no fields to update")
		return
	}
	updates["version"] = m.Version + 1
	database.DB.Model(&m).Updates(updates)
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "memory.update", id, "memory", updates, c.ClientIP())
	database.DB.First(&m, "id = ?", id)
	response.OK(c, m)
}

// Archive archives a memory
func (h *MemoryHandler) Archive(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")
	if err := database.DB.Model(&models.Memory{}).Where("id = ? AND workspace_id = ?", id, wsID).Update("state", "archived").Error; err != nil {
		response.InternalError(c, "archive failed: "+err.Error())
		return
	}
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "memory.archive", id, "memory", nil, c.ClientIP())
	response.OK(c, gin.H{"archived": true})
}

// Review reviews pending memories (candidates scored as pending_review by propose_memory).
// body: {"action": "approve"|"reject", "reason": "..."}
func (h *MemoryHandler) Review(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")

	type req struct {
		Action string `json:"action" binding:"required"`
		Reason string `json:"reason"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if r.Action != "approve" && r.Action != "reject" {
		response.BadRequest(c, "action must be approve or reject")
		return
	}

	var m models.Memory
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).First(&m).Error; err != nil {
		response.NotFound(c, "memory not found")
		return
	}
	if m.State != "pending_review" {
		response.BadRequest(c, "memory is not pending review")
		return
	}

	if r.Action == "reject" {
		database.DB.Model(&m).Update("state", "rejected")
		auditSvc.Log(wsID, middleware.GetUserID(c), "user", "memory.review.reject", id, "memory", r.Reason, c.ClientIP())
		response.OK(c, gin.H{"id": id, "state": "rejected"})
		return
	}

	// approve: enforce active memory quota before approving
	if services.MemoryQuotaExceeded(wsID) {
		response.BadRequest(c, "workspace memory count limit reached; cannot approve")
		return
	}
	database.DB.Model(&m).Update("state", "active")
	now := time.Now()
	database.DB.Create(&models.MemoryValidity{
		MemoryID:    m.ID,
		WorkspaceID: wsID,
		ValidFrom:   now,
		RecordedAt:  now,
	})
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "memory.review.approve", id, "memory", r.Reason, c.ClientIP())
	response.OK(c, gin.H{"id": id, "state": "active"})
}

// Delete permanently deletes a memory
func (h *MemoryHandler) Delete(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).Delete(&models.Memory{}).Error; err != nil {
		response.InternalError(c, "delete failed: "+err.Error())
		return
	}
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "memory.delete", id, "memory", nil, c.ClientIP())
	response.OK(c, gin.H{"deleted": true})
}

// Search performs semantic search
func (h *MemoryHandler) Search(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	userID := middleware.GetUserID(c)
	type req struct {
		Query        string  `json:"query" binding:"required"`
		Scope        string  `json:"scope"`
		ProjectID    *string `json:"project_id"`
		Limit        int     `json:"limit"`
		MinRelevance float64 `json:"min_relevance"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if r.Limit == 0 {
		r.Limit = 10
	}
	if r.Limit > 50 {
		r.Limit = 50
	}
	if r.MinRelevance == 0 {
		r.MinRelevance = 0.1
	}
	if r.Scope == "" {
		r.Scope = "workspace"
	}

	q := database.DB.Where("workspace_id = ? AND state = 'active'", wsID)
	if r.Scope == "project" && r.ProjectID != nil {
		q = q.Where("project_id = ?", *r.ProjectID)
	}
	var candidates []models.Memory
	q.Order("pinned DESC, importance DESC").Limit(200).Find(&candidates)

	type resultItem struct {
		models.Memory
		Relevance float64 `json:"relevance"`
	}
	results := make([]resultItem, 0)
	for _, m := range candidates {
		score := services.Relevance(r.Query, m.Content)
		// Weighting: importance + pinned
		score += m.Importance * 0.25
		if m.Pinned {
			score += 0.15
		}
		score = math.Min(score, 1.0)
		if score >= r.MinRelevance {
			results = append(results, resultItem{Memory: m, Relevance: score})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Relevance > results[j].Relevance
	})
	if len(results) > r.Limit {
		results = results[:r.Limit]
	}

	// Record access
	go func(memResults []resultItem) {
		now := time.Now()
		for _, r := range memResults {
			database.DB.Model(&models.Memory{}).Where("id = ?", r.ID).Updates(map[string]interface{}{
				"access_count":   r.AccessCount + 1,
				"last_access_at": &now,
			})
			database.DB.Create(&models.MemoryAccessLog{
				MemoryID:    r.ID,
				WorkspaceID: wsID,
				UserID:      userID,
				QueryType:   "search",
				Relevance:   r.Relevance,
				AccessedAt:  now,
			})
		}
	}(results)

	usageSvc.Record(wsID, userID, "memory_search", 1)
	response.OK(c, gin.H{
		"query":   r.Query,
		"results": results,
		"count":   len(results),
	})
}

// Stats returns memory statistics
func (h *MemoryHandler) Stats(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)

	type typeCount struct {
		Type  string `json:"type"`
		Count int    `json:"count"`
	}
	var byType []typeCount
	database.DB.Model(&models.Memory{}).
		Select("type, COUNT(*) as count").
		Where("workspace_id = ? AND state = 'active'", wsID).
		Group("type").Scan(&byType)

	var total int64
	database.DB.Model(&models.Memory{}).Where("workspace_id = ? AND state = 'active'", wsID).Count(&total)

	var charSum int64
	database.DB.Model(&models.Memory{}).Where("workspace_id = ? AND state = 'active'", wsID).Select("COALESCE(SUM(char_count), 0)").Scan(&charSum)

	var ws models.Workspace
	database.DB.First(&ws, "id = ?", wsID)

	response.OK(c, gin.H{
		"total_count":     total,
		"total_chars":     charSum,
		"by_type":         byType,
		"quota_max":       ws.QuotaMemoryCount,
		"global_char_max": 2200, // default character limit
	})
}
