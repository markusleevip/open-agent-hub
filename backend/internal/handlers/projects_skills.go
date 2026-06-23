package handlers

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openagenthub/backend/internal/database"
	"github.com/openagenthub/backend/internal/middleware"
	"github.com/openagenthub/backend/internal/models"
	"github.com/openagenthub/backend/internal/response"
	"github.com/openagenthub/backend/internal/services"
	"gorm.io/gorm"
)

// ProjectHandler handles projects
type ProjectHandler struct{}

func NewProjectHandler() *ProjectHandler {
	return &ProjectHandler{}
}

// List lists projects in the current workspace
func (h *ProjectHandler) List(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	var projects []models.Project
	database.DB.Where("workspace_id = ?", wsID).Order("created_at DESC").Find(&projects)
	response.OK(c, gin.H{"items": projects})
}

// Get returns project details
func (h *ProjectHandler) Get(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")
	var p models.Project
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).First(&p).Error; err != nil {
		response.NotFound(c, "project not found")
		return
	}
	response.OK(c, p)
}

// Create creates a project
func (h *ProjectHandler) Create(c *gin.Context) {
	type req struct {
		Name        string `json:"name" binding:"required"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
		Stack       string `json:"stack"`
		Structure   string `json:"structure"`
		RepoName    string `json:"repo_name"`
		GitRemote   string `json:"git_remote"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	orgID := middleware.GetOrgID(c)
	wsID := middleware.GetWorkspaceID(c)
	userID := middleware.GetUserID(c)

	slug := r.Slug
	if slug == "" {
		slug = r.Name
	}
	p := models.Project{
		OrgID:       orgID,
		WorkspaceID: wsID,
		Name:        r.Name,
		Slug:        slug,
		Description: r.Description,
		Stack:       r.Stack,
		Structure:   r.Structure,
		Status:      "active",
		GitRemote:   services.NormalizeGitRemote(r.GitRemote),
		// Project directory: tolerate users entering the full path by extracting the leaf directory name;
		// repo_path is not maintained in the Console; it is auto-written by agent sync.
		RepoName: services.RepoNameFromPath(r.RepoName),
	}
	if err := database.DB.Create(&p).Error; err != nil {
		response.InternalError(c, "create failed: "+err.Error())
		return
	}
	auditSvc.Log(wsID, userID, "user", "project.create", p.ID, "project", r, c.ClientIP())
	response.OK(c, p)
}

// Update updates a project
func (h *ProjectHandler) Update(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")
	var p models.Project
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).First(&p).Error; err != nil {
		response.NotFound(c, "project not found")
		return
	}
	type req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Stack       *string `json:"stack"`
		Structure   *string `json:"structure"`
		RepoName    *string `json:"repo_name"`
		GitRemote   *string `json:"git_remote"`
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
	if r.Stack != nil {
		updates["stack"] = *r.Stack
	}
	if r.Structure != nil {
		updates["structure"] = *r.Structure
	}
	if r.RepoName != nil {
		updates["repo_name"] = services.RepoNameFromPath(*r.RepoName)
	}
	if r.GitRemote != nil {
		updates["git_remote"] = services.NormalizeGitRemote(*r.GitRemote)
	}
	if len(updates) == 0 {
		response.BadRequest(c, "no fields to update")
		return
	}
	database.DB.Model(&p).Updates(updates)
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "project.update", id, "project", updates, c.ClientIP())
	database.DB.First(&p, "id = ?", id)
	response.OK(c, p)
}

// syncRecordView is the display view of a sync record, enriched with the username and
// a "behind current bundle" marker.
type syncRecordView struct {
	models.SyncRecord
	UserDisplayName string `json:"user_display_name"`
	Username        string `json:"username"`
	Stale           bool   `json:"stale"`
}

// SyncRecords lists sync records for a project (multi-user/multi-client observability).
// Returns the most recent sync etag/time/client per endpoint and compares against the current
// bundle etag to mark stale endpoints.
func (h *ProjectHandler) SyncRecords(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	userID := middleware.GetUserID(c)
	id := c.Param("id")

	var p models.Project
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).First(&p).Error; err != nil {
		response.NotFound(c, "project not found")
		return
	}

	var records []models.SyncRecord
	database.DB.Where("project_id = ?", p.ID).Order("synced_at DESC").Find(&records)

	// Current bundle etag: compared against per-endpoint records; stale ones are marked.
	// Note: bundle contains personal data (profile/memories); just render for the caller,
	// used for coarse-grained team-level comparison.
	currentETag := ""
	if bundle, err := services.BuildSyncBundle(wsID, userID, &p); err == nil {
		currentETag = bundle.ETag
	}

	// Batch-fetch usernames to avoid N+1.
	userIDs := make([]string, 0, len(records))
	seen := map[string]bool{}
	for _, r := range records {
		if !seen[r.UserID] {
			seen[r.UserID] = true
			userIDs = append(userIDs, r.UserID)
		}
	}
	type userLite struct {
		ID          string
		DisplayName string
		Username    string
	}
	nameByID := map[string]userLite{}
	if len(userIDs) > 0 {
		var users []userLite
		database.DB.Model(&models.User{}).Select("id", "display_name", "username").
			Where("id IN ?", userIDs).Scan(&users)
		for _, u := range users {
			nameByID[u.ID] = u
		}
	}

	views := make([]syncRecordView, 0, len(records))
	for _, r := range records {
		u := nameByID[r.UserID]
		views = append(views, syncRecordView{
			SyncRecord:      r,
			UserDisplayName: u.DisplayName,
			Username:        u.Username,
			Stale:           currentETag != "" && r.ETag != currentETag,
		})
	}
	// Note: use "records" not "items" for the field name — the frontend axios interceptor
	// smart-unwraps {items:[...]} into an array, losing the sibling current_etag;
	// using "records" lets the whole object pass through intact.
	response.OK(c, gin.H{"records": views, "current_etag": currentETag})
}

// Delete deletes a project
func (h *ProjectHandler) Delete(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).Delete(&models.Project{}).Error; err != nil {
		response.InternalError(c, "delete failed: "+err.Error())
		return
	}
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "project.delete", id, "project", nil, c.ClientIP())
	response.OK(c, gin.H{"deleted": true})
}

// SkillHandler handles Skill management
type SkillHandler struct{}

func NewSkillHandler() *SkillHandler {
	return &SkillHandler{}
}

// List skill list (type=skill memories)
func (h *SkillHandler) List(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	state := c.DefaultQuery("state", "active")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	q := database.DB.Where("workspace_id = ? AND type = 'skill'", wsID)
	if state == "all" {
		// No filter
	} else if state != "" {
		q = q.Where("state = ?", state)
	}
	var total int64
	q.Model(&models.Memory{}).Count(&total)
	var skills []models.Memory
	q.Order("pinned DESC, updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&skills)
	response.OK(c, gin.H{
		"items":     skills,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// CreateSkill create a Skill
func (h *SkillHandler) Create(c *gin.Context) {
	type req struct {
		Content    string  `json:"content" binding:"required"`
		Tags       string  `json:"tags"`
		Importance float64 `json:"importance"`
		Pinned     bool    `json:"pinned"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	orgID := middleware.GetOrgID(c)
	wsID := middleware.GetWorkspaceID(c)
	userID := middleware.GetUserID(c)
	if r.Importance == 0 {
		r.Importance = 0.7
	}
	if r.Tags == "" {
		r.Tags = "[]"
	}
	m := models.Memory{
		OrgID:       orgID,
		WorkspaceID: wsID,
		UserID:      userID,
		Content:     r.Content,
		Type:        "skill",
		Category:    "procedural",
		Tags:        r.Tags,
		Scope:       "workspace",
		Provenance:  "human_curated",
		Importance:  r.Importance,
		Pinned:      r.Pinned,
		State:       "active",
		CharCount:   len([]rune(r.Content)),
	}
	if err := database.DB.Create(&m).Error; err != nil {
		response.InternalError(c, "create failed: "+err.Error())
		return
	}
	auditSvc.Log(wsID, userID, "user", "skill.create", m.ID, "skill", nil, c.ClientIP())
	response.OK(c, m)
}

// ArchiveSkill archive/restore
func (h *SkillHandler) ChangeState(c *gin.Context) {
	id := c.Param("id")
	type req struct {
		State string `json:"state" binding:"required,oneof=active stale archived"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	wsID := middleware.GetWorkspaceID(c)

	var m models.Memory
	if err := database.DB.Where("id = ? AND workspace_id = ? AND type = 'skill'", id, wsID).First(&m).Error; err != nil {
		response.NotFound(c, "skill not found")
		return
	}

	oldState := m.State
	m.State = r.State
	database.DB.Save(&m)

	// Governance log
	database.DB.Create(&models.SkillCurationLog{
		SkillID:   m.ID,
		OldState:  oldState,
		NewState:  r.State,
		Reason:    "manual operation",
		CuratedAt: time.Now(),
	})
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "skill.state_change", id, "skill", gin.H{"from": oldState, "to": r.State}, c.ClientIP())
	response.OK(c, m)
}

// PublicSkillHandler public Skill template directory.
type PublicSkillHandler struct{}

func NewPublicSkillHandler() *PublicSkillHandler {
	return &PublicSkillHandler{}
}

type publicSkillInstallView struct {
	ID               string  `json:"id"`
	WorkspaceID      string  `json:"workspace_id"`
	ProjectID        *string `json:"project_id"`
	TemplateID       string  `json:"template_id"`
	InstalledVersion int     `json:"installed_version"`
	State            string  `json:"state"`
	Pinned           bool    `json:"pinned"`
	InstalledBy      string  `json:"installed_by"`
	InstalledAt      string  `json:"installed_at"`
	UpgradedAt       *string `json:"upgraded_at"`
}

type publicSkillWithInstalls struct {
	models.PublicSkillTemplate
	Installs         []publicSkillInstallView `json:"installs"`
	Installed        bool                     `json:"installed"`
	InstalledVersion int                      `json:"installed_version,omitempty"`
}

type publicSkillUpsertReq struct {
	Slug        string   `json:"slug" binding:"required"`
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Content     string   `json:"content" binding:"required"`
	Category    string   `json:"category" binding:"required"`
	Tags        []string `json:"tags"`
	RiskLevel   string   `json:"risk_level" binding:"required"`
	Status      string   `json:"status" binding:"required"`
}

var publicSkillSlugRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func normalizePublicSkillReq(r *publicSkillUpsertReq) {
	r.Slug = strings.ToLower(strings.TrimSpace(r.Slug))
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	r.Content = strings.TrimSpace(r.Content)
	r.Category = strings.TrimSpace(r.Category)
	r.RiskLevel = strings.TrimSpace(r.RiskLevel)
	r.Status = strings.TrimSpace(r.Status)
	tags := make([]string, 0, len(r.Tags))
	seen := map[string]bool{}
	for _, tag := range r.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	r.Tags = tags
}

func validatePublicSkillReq(r publicSkillUpsertReq) string {
	if !publicSkillSlugRE.MatchString(r.Slug) {
		return "slug must use lowercase letters, numbers and hyphens"
	}
	if r.Name == "" {
		return "name is required"
	}
	if r.Content == "" {
		return "content is required"
	}
	if r.Category == "" {
		return "category is required"
	}
	switch r.RiskLevel {
	case "low", "medium", "high":
	default:
		return "risk_level must be one of low, medium, high"
	}
	switch r.Status {
	case "draft", "active", "archived":
	default:
		return "status must be one of draft, active, archived"
	}
	return ""
}

func publicSkillTagsJSON(tags []string) (string, error) {
	if tags == nil {
		tags = []string{}
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func installView(si models.SkillInstall) publicSkillInstallView {
	var upgradedAt *string
	if si.UpgradedAt != nil {
		v := si.UpgradedAt.Format(time.RFC3339)
		upgradedAt = &v
	}
	return publicSkillInstallView{
		ID:               si.ID,
		WorkspaceID:      si.WorkspaceID,
		ProjectID:        si.ProjectID,
		TemplateID:       si.TemplateID,
		InstalledVersion: si.InstalledVersion,
		State:            si.State,
		Pinned:           si.Pinned,
		InstalledBy:      si.InstalledBy,
		InstalledAt:      si.InstalledAt.Format(time.RFC3339),
		UpgradedAt:       upgradedAt,
	}
}

// Create create a public Skill template. Only owner/admin can call; route layer handles permission checks.
func (h *PublicSkillHandler) Create(c *gin.Context) {
	var r publicSkillUpsertReq
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	normalizePublicSkillReq(&r)
	if msg := validatePublicSkillReq(r); msg != "" {
		response.BadRequest(c, msg)
		return
	}

	var existing models.PublicSkillTemplate
	if err := database.DB.Unscoped().First(&existing, "slug = ?", r.Slug).Error; err == nil {
		response.Error(c, 409, "public skill slug already exists")
		return
	} else if err != gorm.ErrRecordNotFound {
		response.InternalError(c, "check slug failed: "+err.Error())
		return
	}

	tags, err := publicSkillTagsJSON(r.Tags)
	if err != nil {
		response.BadRequest(c, "invalid tags: "+err.Error())
		return
	}
	tpl := models.PublicSkillTemplate{
		Slug:        r.Slug,
		Name:        r.Name,
		Description: r.Description,
		Content:     r.Content,
		Category:    r.Category,
		Tags:        tags,
		Version:     1,
		RiskLevel:   r.RiskLevel,
		Visibility:  "public",
		Source:      "manual",
		Status:      r.Status,
	}
	if err := database.DB.Create(&tpl).Error; err != nil {
		response.InternalError(c, "create public skill failed: "+err.Error())
		return
	}
	auditSvc.Log(middleware.GetWorkspaceID(c), middleware.GetUserID(c), "user", "public_skill.create", tpl.ID, "public_skill", gin.H{
		"slug":   tpl.Slug,
		"status": tpl.Status,
	}, c.ClientIP())
	response.OK(c, tpl)
}

// Update modify a public Skill template. Increment template version when content changes; installed instances still retain the install snapshot.
func (h *PublicSkillHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var tpl models.PublicSkillTemplate
	if err := database.DB.First(&tpl, "id = ?", id).Error; err != nil {
		response.NotFound(c, "public skill not found")
		return
	}

	var r publicSkillUpsertReq
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	normalizePublicSkillReq(&r)
	if msg := validatePublicSkillReq(r); msg != "" {
		response.BadRequest(c, msg)
		return
	}
	if tpl.Slug != r.Slug {
		var existing models.PublicSkillTemplate
		if err := database.DB.Unscoped().Where("id <> ?", tpl.ID).First(&existing, "slug = ?", r.Slug).Error; err == nil {
			response.Error(c, 409, "public skill slug already exists")
			return
		} else if err != gorm.ErrRecordNotFound {
			response.InternalError(c, "check slug failed: "+err.Error())
			return
		}
	}

	tags, err := publicSkillTagsJSON(r.Tags)
	if err != nil {
		response.BadRequest(c, "invalid tags: "+err.Error())
		return
	}
	oldVersion := tpl.Version
	if tpl.Content != r.Content {
		tpl.Version++
	}
	tpl.Slug = r.Slug
	tpl.Name = r.Name
	tpl.Description = r.Description
	tpl.Content = r.Content
	tpl.Category = r.Category
	tpl.Tags = tags
	tpl.RiskLevel = r.RiskLevel
	tpl.Status = r.Status
	if err := database.DB.Save(&tpl).Error; err != nil {
		response.InternalError(c, "update public skill failed: "+err.Error())
		return
	}
	auditSvc.Log(middleware.GetWorkspaceID(c), middleware.GetUserID(c), "user", "public_skill.update", tpl.ID, "public_skill", gin.H{
		"slug":         tpl.Slug,
		"old_version":  oldVersion,
		"new_version":  tpl.Version,
		"content_bump": oldVersion != tpl.Version,
	}, c.ClientIP())
	response.OK(c, tpl)
}

// ChangeStatus toggle public Skill template status.
func (h *PublicSkillHandler) ChangeStatus(c *gin.Context) {
	id := c.Param("id")
	type req struct {
		Status string `json:"status" binding:"required"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	r.Status = strings.TrimSpace(r.Status)
	switch r.Status {
	case "draft", "active", "archived":
	default:
		response.BadRequest(c, "status must be one of draft, active, archived")
		return
	}

	var tpl models.PublicSkillTemplate
	if err := database.DB.First(&tpl, "id = ?", id).Error; err != nil {
		response.NotFound(c, "public skill not found")
		return
	}
	oldStatus := tpl.Status
	tpl.Status = r.Status
	if err := database.DB.Save(&tpl).Error; err != nil {
		response.InternalError(c, "update public skill status failed: "+err.Error())
		return
	}
	auditSvc.Log(middleware.GetWorkspaceID(c), middleware.GetUserID(c), "user", "public_skill.status_change", tpl.ID, "public_skill", gin.H{
		"from": oldStatus,
		"to":   tpl.Status,
	}, c.ClientIP())
	response.OK(c, tpl)
}

// List public Skill templates, along with the current workspace install status.
func (h *PublicSkillHandler) List(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	category := c.Query("category")
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := c.DefaultQuery("status", "active")
	installed := c.Query("installed")

	q := database.DB.Model(&models.PublicSkillTemplate{})
	if status != "all" && status != "" {
		q = q.Where("status = ?", status)
	}
	if category != "" && category != "all" {
		q = q.Where("category = ?", category)
	}
	if keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		q = q.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ? OR LOWER(tags) LIKE ?", like, like, like)
	}

	var templates []models.PublicSkillTemplate
	if err := q.Order("category ASC, name ASC").Find(&templates).Error; err != nil {
		response.InternalError(c, "list public skills failed: "+err.Error())
		return
	}

	ids := make([]string, 0, len(templates))
	for _, tpl := range templates {
		ids = append(ids, tpl.ID)
	}

	installsByTemplate := map[string][]publicSkillInstallView{}
	if len(ids) > 0 {
		var installs []models.SkillInstall
		database.DB.Where("workspace_id = ? AND template_id IN ?", wsID, ids).
			Order("project_id ASC, created_at ASC").Find(&installs)
		for _, si := range installs {
			installsByTemplate[si.TemplateID] = append(installsByTemplate[si.TemplateID], installView(si))
		}
	}

	out := make([]publicSkillWithInstalls, 0, len(templates))
	for _, tpl := range templates {
		installs := installsByTemplate[tpl.ID]
		isInstalled := len(installs) > 0
		if installed == "true" && !isInstalled {
			continue
		}
		if installed == "false" && isInstalled {
			continue
		}
		item := publicSkillWithInstalls{PublicSkillTemplate: tpl, Installs: installs, Installed: isInstalled}
		if isInstalled {
			item.InstalledVersion = installs[0].InstalledVersion
		}
		out = append(out, item)
	}
	response.OK(c, gin.H{"items": out})
}

// Get return public Skill details and the current workspace install record.
func (h *PublicSkillHandler) Get(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	id := c.Param("id")

	var tpl models.PublicSkillTemplate
	if err := database.DB.First(&tpl, "id = ?", id).Error; err != nil {
		response.NotFound(c, "public skill not found")
		return
	}

	var installs []models.SkillInstall
	database.DB.Where("workspace_id = ? AND template_id = ?", wsID, tpl.ID).Order("created_at ASC").Find(&installs)
	views := make([]publicSkillInstallView, 0, len(installs))
	for _, si := range installs {
		views = append(views, installView(si))
	}

	response.OK(c, publicSkillWithInstalls{
		PublicSkillTemplate: tpl,
		Installs:            views,
		Installed:           len(views) > 0,
	})
}

// SkillInstallHandler public Skill install relationships.
type SkillInstallHandler struct{}

func NewSkillInstallHandler() *SkillInstallHandler {
	return &SkillInstallHandler{}
}

type skillInstallDetail struct {
	models.SkillInstall
	Template models.PublicSkillTemplate `json:"template"`
	Project  *models.Project            `json:"project,omitempty"`
}

// List public Skills installed in the current workspace.
func (h *SkillInstallHandler) List(c *gin.Context) {
	wsID := middleware.GetWorkspaceID(c)
	state := c.Query("state")
	projectID := c.Query("project_id")

	q := database.DB.Where("workspace_id = ?", wsID)
	if state != "" && state != "all" {
		q = q.Where("state = ?", state)
	}
	if projectID != "" {
		if projectID == "workspace" {
			q = q.Where("project_id IS NULL")
		} else {
			q = q.Where("project_id = ?", projectID)
		}
	}

	var installs []models.SkillInstall
	if err := q.Order("pinned DESC, created_at DESC").Find(&installs).Error; err != nil {
		response.InternalError(c, "list skill installs failed: "+err.Error())
		return
	}
	response.OK(c, gin.H{"items": enrichSkillInstalls(installs)})
}

func enrichSkillInstalls(installs []models.SkillInstall) []skillInstallDetail {
	out := make([]skillInstallDetail, 0, len(installs))
	for _, si := range installs {
		detail := skillInstallDetail{SkillInstall: si}
		database.DB.First(&detail.Template, "id = ?", si.TemplateID)
		if si.ProjectID != nil {
			var p models.Project
			if err := database.DB.First(&p, "id = ?", *si.ProjectID).Error; err == nil {
				detail.Project = &p
			}
		}
		out = append(out, detail)
	}
	return out
}

// Create install a public Skill into a workspace or project.
func (h *SkillInstallHandler) Create(c *gin.Context) {
	type req struct {
		TemplateID string  `json:"template_id" binding:"required"`
		ProjectID  *string `json:"project_id"`
		Pinned     bool    `json:"pinned"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	wsID := middleware.GetWorkspaceID(c)
	userID := middleware.GetUserID(c)

	var tpl models.PublicSkillTemplate
	if err := database.DB.Where("id = ? AND status = ?", r.TemplateID, "active").First(&tpl).Error; err != nil {
		response.NotFound(c, "active public skill not found")
		return
	}
	if tpl.RiskLevel == "high" {
		response.Forbidden(c, "high-risk public skills cannot be installed in P0")
		return
	}

	var projectID *string
	if r.ProjectID != nil && strings.TrimSpace(*r.ProjectID) != "" {
		pid := strings.TrimSpace(*r.ProjectID)
		var p models.Project
		if err := database.DB.Where("id = ? AND workspace_id = ?", pid, wsID).First(&p).Error; err != nil {
			response.BadRequest(c, "project not found in current workspace")
			return
		}
		projectID = &pid
	}

	dup := database.DB.Where("workspace_id = ? AND template_id = ?", wsID, tpl.ID)
	if projectID == nil {
		dup = dup.Where("project_id IS NULL")
	} else {
		dup = dup.Where("project_id = ?", *projectID)
	}
	var existing models.SkillInstall
	if err := dup.First(&existing).Error; err == nil {
		if existing.State == "archived" {
			now := time.Now()
			oldVersion := existing.InstalledVersion
			existing.State = "active"
			existing.Pinned = r.Pinned
			existing.InstalledVersion = tpl.Version
			existing.OverrideContent = tpl.Content
			existing.InstalledBy = userID
			existing.InstalledAt = now
			existing.UpgradedAt = nil
			if err := database.DB.Save(&existing).Error; err != nil {
				response.InternalError(c, "reactivate public skill install failed: "+err.Error())
				return
			}
			auditSvc.Log(wsID, userID, "user", "skill.install_reactivate", existing.ID, "skill_install", gin.H{
				"template_id": tpl.ID,
				"project_id":  projectID,
				"from":        oldVersion,
				"to":          tpl.Version,
			}, c.ClientIP())
			response.OK(c, enrichSkillInstalls([]models.SkillInstall{existing})[0])
			return
		}
		response.Error(c, 409, "public skill already installed in this scope")
		return
	} else if err != gorm.ErrRecordNotFound {
		response.InternalError(c, "check duplicate install failed: "+err.Error())
		return
	}

	now := time.Now()
	install := models.SkillInstall{
		WorkspaceID:      wsID,
		ProjectID:        projectID,
		TemplateID:       tpl.ID,
		InstalledVersion: tpl.Version,
		State:            "active",
		Pinned:           r.Pinned,
		OverrideContent:  tpl.Content,
		InstalledBy:      userID,
		InstalledAt:      now,
	}
	if err := database.DB.Create(&install).Error; err != nil {
		response.InternalError(c, "install public skill failed: "+err.Error())
		return
	}
	auditSvc.Log(wsID, userID, "user", "skill.install", install.ID, "skill_install", gin.H{
		"template_id": tpl.ID,
		"project_id":  projectID,
	}, c.ClientIP())
	response.OK(c, enrichSkillInstalls([]models.SkillInstall{install})[0])
}

// ChangeState enable/disable/archive an install relationship.
func (h *SkillInstallHandler) ChangeState(c *gin.Context) {
	id := c.Param("id")
	type req struct {
		State string `json:"state" binding:"required,oneof=active disabled archived"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	wsID := middleware.GetWorkspaceID(c)
	var install models.SkillInstall
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).First(&install).Error; err != nil {
		response.NotFound(c, "skill install not found")
		return
	}
	oldState := install.State
	install.State = r.State
	if err := database.DB.Save(&install).Error; err != nil {
		response.InternalError(c, "update install state failed: "+err.Error())
		return
	}
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "skill.install_state_change", id, "skill_install", gin.H{
		"from": oldState,
		"to":   r.State,
	}, c.ClientIP())
	response.OK(c, enrichSkillInstalls([]models.SkillInstall{install})[0])
}

// Upgrade upgrade the installed version to the current template version.
func (h *SkillInstallHandler) Upgrade(c *gin.Context) {
	id := c.Param("id")
	wsID := middleware.GetWorkspaceID(c)

	var install models.SkillInstall
	if err := database.DB.Where("id = ? AND workspace_id = ?", id, wsID).First(&install).Error; err != nil {
		response.NotFound(c, "skill install not found")
		return
	}
	var tpl models.PublicSkillTemplate
	if err := database.DB.First(&tpl, "id = ?", install.TemplateID).Error; err != nil {
		response.NotFound(c, "public skill template not found")
		return
	}
	now := time.Now()
	oldVersion := install.InstalledVersion
	install.InstalledVersion = tpl.Version
	install.OverrideContent = tpl.Content
	install.UpgradedAt = &now
	if err := database.DB.Save(&install).Error; err != nil {
		response.InternalError(c, "upgrade install failed: "+err.Error())
		return
	}
	auditSvc.Log(wsID, middleware.GetUserID(c), "user", "skill.install_upgrade", id, "skill_install", gin.H{
		"from": oldVersion,
		"to":   tpl.Version,
	}, c.ClientIP())
	response.OK(c, enrichSkillInstalls([]models.SkillInstall{install})[0])
}
