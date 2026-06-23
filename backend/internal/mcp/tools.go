package mcp

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/openagenthub/backend/internal/database"
	"github.com/openagenthub/backend/internal/models"
	"github.com/openagenthub/backend/internal/services"
)

// Context is the MCP call context (injected after authentication)
type Context struct {
	WorkspaceID   string
	UserID        string
	OrgID         string
	Role          string
	AgentClientID string
	MCPSessionID  string
	ClientIP      string
	UserAgent     string
	// ProjectID request-level project binding: resolved by the gateway from the X-Project-Path header
	// (see resolveProjectBinding); used as a fallback when the tool does not explicitly pass project_id.
	ProjectID string
}

// ToolHandler is a Tool handler function
type ToolHandler func(ctx *Context, args map[string]interface{}) (interface{}, error)

// ToolRegistry is the tool registry
type ToolRegistry struct {
	tools    map[string]Tool
	handlers map[string]ToolHandler
}

type availableSkill struct {
	Source      string   `json:"source"`
	ID          string   `json:"id,omitempty"`
	MemoryID    string   `json:"memory_id,omitempty"`
	TemplateID  string   `json:"template_id,omitempty"`
	InstallID   string   `json:"install_id,omitempty"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Content     string   `json:"content"`
	Version     int      `json:"version"`
	Tags        []string `json:"tags"`
	Pinned      bool     `json:"pinned"`
	Relevance   float64  `json:"relevance,omitempty"`
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:    make(map[string]Tool),
		handlers: make(map[string]ToolHandler),
	}
}

func (r *ToolRegistry) Register(tool Tool, handler ToolHandler) {
	r.tools[tool.Name] = tool
	r.handlers[tool.Name] = handler
}

func (r *ToolRegistry) Get(name string) (Tool, ToolHandler, bool) {
	t, ok := r.tools[name]
	if !ok {
		return Tool{}, nil, false
	}
	return t, r.handlers[name], true
}

func (r *ToolRegistry) List() []Tool {
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Generic: string field
func strArg(args map[string]interface{}, k string, def string) string {
	if v, ok := args[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func intArg(args map[string]interface{}, k string, def int) int {
	if v, ok := args[k]; ok {
		switch t := v.(type) {
		case float64:
			return int(t)
		case int:
			return t
		case string:
			n, _ := strconv.Atoi(t)
			if n != 0 {
				return n
			}
		}
	}
	return def
}

func floatArg(args map[string]interface{}, k string, def float64) float64 {
	if v, ok := args[k]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return def
}

func boolArg(args map[string]interface{}, k string, def bool) bool {
	if v, ok := args[k]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

func strPtrArg(args map[string]interface{}, k string) *string {
	if v, ok := args[k]; ok {
		if s, ok := v.(string); ok && s != "" {
			return &s
		}
	}
	return nil
}

// Retrieval scoring logic is centralized in internal/services (Tokenize/Relevance/Similarity),
// shared by MCP and REST.

// containsAny checks for substring containment
func containsAny(text, query string) bool {
	textLower := strings.ToLower(text)
	qWords := strings.Fields(strings.ToLower(query))
	for _, w := range qWords {
		if strings.Contains(textLower, w) {
			return true
		}
	}
	return false
}

// computeETag computes an etag
func computeETag(parts ...string) string {
	h := md5.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// jsonText returns a JSON string
func jsonText(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func parseTagsJSON(s string) []string {
	var tags []string
	if s != "" {
		_ = json.Unmarshal([]byte(s), &tags)
	}
	if tags == nil {
		return []string{}
	}
	return tags
}

func skillNameAndDescription(content string) (string, string) {
	name, description := "", ""
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if name == "" {
				name = strings.TrimSpace(strings.TrimLeft(line, "# "))
			}
			continue
		}
		if name == "" {
			name = line
		} else if description == "" {
			description = line
		}
		if name != "" && description != "" {
			break
		}
	}
	if name == "" {
		name = "Untitled Skill"
	}
	if description == "" {
		description = name
	}
	if r := []rune(name); len(r) > 80 {
		name = string(r[:80])
	}
	if r := []rune(description); len(r) > 200 {
		description = string(r[:200])
	}
	return name, description
}

func availableSkills(ctx *Context, projectID string, source string, limit int) []availableSkill {
	if limit <= 0 {
		limit = 50
	}
	if source == "" {
		source = "all"
	}

	out := []availableSkill{}

	if source == "all" || source == "custom" {
		q := database.DB.Where("workspace_id = ? AND type = ? AND state = ?", ctx.WorkspaceID, "skill", "active")
		if projectID != "" {
			q = q.Where("project_id IS NULL OR project_id = ?", projectID)
		} else {
			q = q.Where("project_id IS NULL")
		}
		var skills []models.Memory
		q.Order("pinned DESC, importance DESC, updated_at DESC").Limit(limit).Find(&skills)
		for _, s := range skills {
			name, description := skillNameAndDescription(s.Content)
			out = append(out, availableSkill{
				Source:      "custom",
				ID:          s.ID,
				MemoryID:    s.ID,
				Name:        name,
				Description: description,
				Content:     s.Content,
				Version:     s.Version,
				Tags:        parseTagsJSON(s.Tags),
				Pinned:      s.Pinned,
			})
		}
	}

	if source == "all" || source == "public" {
		q := database.DB.Where("workspace_id = ? AND state = ?", ctx.WorkspaceID, "active")
		if projectID != "" {
			q = q.Where("project_id IS NULL OR project_id = ?", projectID)
		} else {
			q = q.Where("project_id IS NULL")
		}
		var installs []models.SkillInstall
		q.Order("pinned DESC, created_at ASC, id ASC").Limit(limit).Find(&installs)
		for _, install := range installs {
			var tpl models.PublicSkillTemplate
			if err := database.DB.Where("id = ? AND status = ?", install.TemplateID, "active").First(&tpl).Error; err != nil {
				continue
			}
			content := install.OverrideContent
			if content == "" {
				content = tpl.Content
			}
			out = append(out, availableSkill{
				Source:      "public",
				ID:          install.ID,
				TemplateID:  tpl.ID,
				InstallID:   install.ID,
				Name:        tpl.Name,
				Description: tpl.Description,
				Content:     content,
				Version:     install.InstalledVersion,
				Tags:        parseTagsJSON(tpl.Tags),
				Pinned:      install.Pinned,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// ============================================================
// 14 P0 tool implementations
// ============================================================

// resolveAgentName resolves the target agent name for the current request, used for
// three-level config override resolution:
// prefer the explicit agent_name parameter; otherwise fall back to the onboarded AgentClient's client_type.
// Returns nil to indicate agent-agnostic resolution (only rules with empty agent_name apply).
func resolveAgentName(ctx *Context, args map[string]interface{}) *string {
	if v := strArg(args, "agent_name", ""); v != "" {
		return &v
	}
	if ctx.AgentClientID != "" {
		var client models.AgentClient
		if database.DB.Select("client_type").First(&client, "id = ?", ctx.AgentClientID).Error == nil && client.ClientType != "" {
			ct := client.ClientType
			return &ct
		}
	}
	return nil
}

// RegisterP0Tools registers P0 tools
func RegisterP0Tools(r *ToolRegistry) {
	// 1. hub.get_agent_profile
	r.Register(Tool{
		Name:        "hub.get_agent_profile",
		Description: "Get the current agent client connection profile",
		InputSchema: InputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		var client models.AgentClient
		if ctx.AgentClientID != "" {
			database.DB.First(&client, "id = ?", ctx.AgentClientID)
		} else {
			// Agent Profile is per-user private, decoupled from workspaces
			database.DB.Where("user_id = ?", ctx.UserID).Order("last_seen_at DESC").First(&client)
		}
		var ws models.Workspace
		database.DB.First(&ws, "id = ?", ctx.WorkspaceID)

		plan := "free"
		if ws.QuotaMemoryCount >= 10000 {
			plan = "pro"
		}

		return map[string]interface{}{
			"agent_client_id": client.ID,
			"client_type":     client.ClientType,
			"client_name":     client.ClientName,
			"client_version":  client.ClientVersion,
			"workspace_id":    ctx.WorkspaceID,
			"user_id":         ctx.UserID,
			"plan":            plan,
		}, nil
	})

	// 2. hub.get_global_rules
	r.Register(Tool{
		Name:        "hub.get_global_rules",
		Description: "Get the workspace global rules (resolved with per-agent overrides)",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"format":     map[string]interface{}{"type": "string", "enum": []string{"markdown", "json"}},
				"agent_name": map[string]interface{}{"type": "string", "description": "Target agent name (optional); defaults to the connecting client_type"},
			},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		// workspace global resolution: project dimension is empty, agent dimension overrides by target agent
		agentName := resolveAgentName(ctx, args)
		effective := services.ResolveEffectiveRules(ctx.WorkspaceID, nil, agentName)

		version := time.Now().UTC().Format(time.RFC3339)

		return map[string]interface{}{
			"rules":   services.RuleValues(effective),
			"detail":  effective,
			"version": version,
			"etag":    services.RuleETag(effective),
		}, nil
	})

	// 3. hub.get_project_rules
	r.Register(Tool{
		Name:        "hub.get_project_rules",
		Description: "Get the effective rules for the current project (global→project→agent override merge)",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"project_id":   map[string]interface{}{"type": "string", "description": "Project id (optional); takes precedence over project_path"},
				"project_path": projectPathProp,
				"agent_name":   map[string]interface{}{"type": "string", "description": "Target agent name (optional); defaults to the connecting client_type"},
			},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		projectID, err := resolveProjectID(ctx, args)
		if err != nil {
			return nil, err
		}
		if projectID == "" {
			return nil, errNoProjectBound()
		}
		agentName := resolveAgentName(ctx, args)

		// Effective rules: override merge within the (project, agent) context
		effective := services.ResolveEffectiveRules(ctx.WorkspaceID, &projectID, agentName)

		// Original layers (for debugging/display only, to visualize override origins)
		var projectRules []models.Rule
		database.DB.Where("workspace_id = ? AND project_id = ?", ctx.WorkspaceID, projectID).Find(&projectRules)
		var globalRules []models.Rule
		database.DB.Where("workspace_id = ? AND project_id IS NULL", ctx.WorkspaceID).Find(&globalRules)

		return map[string]interface{}{
			"effective_rules": services.RuleValues(effective),
			"effective":       effective,
			"project_rules":   projectRules,
			"global_rules":    globalRules,
			"etag":            services.RuleETag(effective),
		}, nil
	})

	// 4. hub.get_workspace_policy
	r.Register(Tool{
		Name:        "hub.get_workspace_policy",
		Description: "Get the overall workspace policy (tool policy set + quotas)",
		InputSchema: InputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		var ws models.Workspace
		database.DB.First(&ws, "id = ?", ctx.WorkspaceID)
		var policies []models.ToolPolicy
		database.DB.Where("workspace_id = ?", ctx.WorkspaceID).Find(&policies)

		today := time.Now().Format("2006-01-02")
		var todayCount int64
		database.DB.Model(&models.UsageRecord{}).Where("workspace_id = ? AND period = ?", ctx.WorkspaceID, today).Select("COALESCE(SUM(quantity), 0)").Scan(&todayCount)

		return map[string]interface{}{
			"tool_policies": policies,
			"quotas": map[string]interface{}{
				"memory_count_max":    ws.QuotaMemoryCount,
				"tool_call_daily_max": ws.QuotaToolCallDaily,
			},
			"today_usage": todayCount,
		}, nil
	})

	// 5. hub.search_memory
	r.Register(Tool{
		Name:        "hub.search_memory",
		Description: "Semantic search over memories",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query":         map[string]interface{}{"type": "string"},
				"scope":         map[string]interface{}{"type": "string", "enum": []string{"workspace", "project", "agent"}},
				"project_id":    map[string]interface{}{"type": "string", "description": "Project id (optional); takes precedence over project_path"},
				"project_path":  projectPathProp,
				"limit":         map[string]interface{}{"type": "integer"},
				"min_relevance": map[string]interface{}{"type": "number"},
			},
			Required: []string{"query"},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		query := strArg(args, "query", "")
		if query == "" {
			return nil, fmt.Errorf("query is required")
		}
		scope := strArg(args, "scope", "workspace")
		limit := intArg(args, "limit", 10)
		if limit > 50 {
			limit = 50
		}
		minRel := floatArg(args, "min_relevance", 0.1)

		q := database.DB.Where("workspace_id = ? AND state = 'active'", ctx.WorkspaceID)
		if scope == "project" {
			projectID, err := resolveProjectID(ctx, args)
			if err != nil {
				return nil, err
			}
			if projectID == "" {
				return nil, errNoProjectBound()
			}
			q = q.Where("project_id = ?", projectID)
		}

		var candidates []models.Memory
		q.Order("pinned DESC, importance DESC").Limit(200).Find(&candidates)

		type item struct {
			ID         string    `json:"id"`
			Content    string    `json:"content"`
			Importance float64   `json:"importance"`
			Scope      string    `json:"scope"`
			Type       string    `json:"type"`
			Relevance  float64   `json:"relevance"`
			CreatedAt  time.Time `json:"created_at"`
		}
		results := make([]item, 0)
		for _, m := range candidates {
			score := services.Relevance(query, m.Content)
			score += m.Importance * 0.25
			if m.Pinned {
				score += 0.15
			}
			if containsAny(m.Tags, query) {
				score += 0.1
			}
			score = math.Min(score, 1.0)
			if score >= minRel {
				results = append(results, item{
					ID: m.ID, Content: m.Content, Importance: m.Importance,
					Scope: m.Scope, Type: m.Type, Relevance: score, CreatedAt: m.CreatedAt,
				})
			}
		}
		sort.Slice(results, func(i, j int) bool { return results[i].Relevance > results[j].Relevance })
		if len(results) > limit {
			results = results[:limit]
		}

		// Log access asynchronously
		go func(results []item) {
			now := time.Now()
			for _, r := range results {
				database.DB.Model(&models.Memory{}).Where("id = ?", r.ID).UpdateColumns(map[string]interface{}{
					"access_count":   gormExprAdd(),
					"last_access_at": &now,
				})
				database.DB.Create(&models.MemoryAccessLog{
					MemoryID: r.ID, WorkspaceID: ctx.WorkspaceID, UserID: ctx.UserID,
					QueryType: "search", Relevance: r.Relevance, AccessedAt: now,
				})
			}
		}(results)

		database.DB.Create(&models.UsageRecord{
			WorkspaceID: ctx.WorkspaceID, UserID: ctx.UserID, Metric: "memory_search",
			Quantity: 1, Period: time.Now().Format("2006-01-02"), RecordedAt: time.Now(),
		})

		if ctx.MCPSessionID != "" {
			var existing models.MemorySnapshot
			if database.DB.Where("session_id = ?", ctx.MCPSessionID).First(&existing).Error != nil {
				allActiveMemJSON, _ := json.Marshal(results)
				snap := models.MemorySnapshot{
					WorkspaceID: ctx.WorkspaceID,
					SessionID:   ctx.MCPSessionID,
					Content:     string(allActiveMemJSON),
					CharCount:   len(allActiveMemJSON),
					MemoryIDs:   "[]",
					Version:     time.Now().UTC().Format("20060102-150405"),
					FrozenAt:    time.Now(),
				}
				database.DB.Create(&snap)
			}
		}

		return map[string]interface{}{
			"query":    query,
			"memories": results,
			"count":    len(results),
		}, nil
	})

	// 6. hub.get_relevant_memory
	r.Register(Tool{
		Name:        "hub.get_relevant_memory",
		Description: "Automatically retrieve relevant memories from the current session context (no query needed)",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"context_summary": map[string]interface{}{"type": "string"},
				"project_path":    projectPathProp,
				"limit":           map[string]interface{}{"type": "integer"},
			},
			Required: []string{"context_summary"},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		summary := strArg(args, "context_summary", "")
		limit := intArg(args, "limit", 5)
		if limit > 20 {
			limit = 20
		}
		if summary == "" {
			return map[string]interface{}{"memories": []interface{}{}, "count": 0}, nil
		}

		q := database.DB.Where("workspace_id = ? AND state = 'active'", ctx.WorkspaceID)
		// When there is a project binding, exclude memories from other projects (keep this project + workspace level)
		if projectID, err := resolveProjectID(ctx, args); err != nil {
			return nil, err
		} else if projectID != "" {
			q = q.Where("project_id = ? OR project_id IS NULL OR project_id = ''", projectID)
		}

		var candidates []models.Memory
		q.Order("pinned DESC, importance DESC, access_count DESC").Limit(100).Find(&candidates)

		type item struct {
			ID         string  `json:"id"`
			Content    string  `json:"content"`
			Type       string  `json:"type"`
			Importance float64 `json:"importance"`
			Relevance  float64 `json:"relevance"`
		}
		results := make([]item, 0)
		for _, m := range candidates {
			score := services.Relevance(summary, m.Content)
			score += m.Importance * 0.3
			if m.Pinned {
				score += 0.2
			}
			score = math.Min(score, 1.0)
			if score >= 0.15 {
				results = append(results, item{
					ID: m.ID, Content: m.Content, Type: m.Type,
					Importance: m.Importance, Relevance: score,
				})
			}
		}
		sort.Slice(results, func(i, j int) bool { return results[i].Relevance > results[j].Relevance })
		if len(results) > limit {
			results = results[:limit]
		}
		return map[string]interface{}{"memories": results, "count": len(results)}, nil
	})

	// 7. hub.propose_memory
	r.Register(Tool{
		Name:        "hub.propose_memory",
		Description: "Propose a memory candidate (the required entry point); after write-discipline scoring it may be auto-accepted or queued for review",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Memory content; must be valid text, max 2200 characters",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Memory type",
					"enum":        []string{"fact", "semantic", "skill", "user_preference"},
				},
				"scope": map[string]interface{}{
					"type":        "string",
					"description": "Scope; for \"project\" the memory is attached to the bound project (bind via project_path or hub.sync_project)",
					"enum":        []string{"workspace", "project"},
				},
				"project_path": projectPathProp,
				"project_id":   map[string]interface{}{"type": "string", "description": "Project id (optional); takes precedence over project_path"},
				"source_agent": map[string]interface{}{
					"type":        "string",
					"description": "Source agent name (optional)",
				},
				"confidence": map[string]interface{}{
					"type":        "number",
					"description": "Confidence 0-1, default 0.8",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Tag array (optional), e.g. [\"project\", \"architecture\"]",
				},
			},
			Required: []string{"content"},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		content := strArg(args, "content", "")
		if content == "" {
			return nil, fmt.Errorf("content is required")
		}
		memType := strArg(args, "type", "semantic")
		scope := strArg(args, "scope", "workspace")
		sourceAgent := strArg(args, "source_agent", "")
		confidence := floatArg(args, "confidence", 0.8)

		projectID, err := resolveProjectID(ctx, args)
		if err != nil {
			return nil, err
		}
		if scope == "project" && projectID == "" {
			return nil, errNoProjectBound()
		}

		tagsArr, _ := args["tags"].([]interface{})
		tagsJSON := "[]"
		if len(tagsArr) > 0 {
			tagsJSON = jsonText(tagsArr)
		}

		// Ingestion discipline evaluation
		decision := "accepted"
		reason := "passed basic quality checks"
		importance := 0.5
		provenance := "agent_extracted"

		// Simple rules:
		// - content too short -> pending_review
		// - confidence < 0.5 -> pending_review
		// - otherwise accepted
		if len([]rune(content)) < 5 {
			decision = "rejected"
			reason = "content too short (< 5 characters)"
		} else if confidence < 0.5 {
			decision = "pending_review"
			reason = "confidence too low (< 0.5); user confirmation required"
		} else if containsAny(content, "TODO|FIXME|XXX|temp|debug") {
			decision = "pending_review"
			reason = "content contains temporary markers; user confirmation recommended"
			importance = 0.3
		} else {
			// High-quality memory
			if memType == "user_preference" {
				importance = 0.85
			} else if memType == "skill" {
				importance = 0.75
			} else {
				importance = 0.6
			}
		}

		// Type to category mapping
		category := "declarative"
		if memType == "skill" {
			category = "procedural"
		}

		// Check for semantic duplicates
		var existing []models.Memory
		database.DB.Where("workspace_id = ? AND state = 'active'", ctx.WorkspaceID).Limit(200).Find(&existing)
		for _, m := range existing {
			if services.Similarity(content, m.Content) >= 0.92 {
				decision = "rejected"
				reason = "semantic duplicate (too similar to an existing memory)"
				return map[string]interface{}{
					"decision":    decision,
					"existing_id": m.ID,
					"reason":      reason,
				}, nil
			}
		}

		// Character count limit
		if len([]rune(content)) > 2200 {
			decision = "rejected"
			reason = "memory content exceeds the 2200-character limit"
		}

		if decision == "rejected" {
			return map[string]interface{}{
				"decision": decision,
				"reason":   reason,
			}, nil
		}

		// accepted -> active (takes effect immediately); pending_review -> persisted for manual review
		targetState := "active"
		if decision == "pending_review" {
			targetState = "pending_review"
		}

		// Only enforce count quota for memories that will take effect immediately (pending review memories do not count against the active quota)
		if targetState == "active" && services.MemoryQuotaExceeded(ctx.WorkspaceID) {
			return map[string]interface{}{
				"decision": "rejected",
				"reason":   "workspace memory count limit reached",
			}, nil
		}

		m := models.Memory{
			OrgID:       ctx.OrgID,
			WorkspaceID: ctx.WorkspaceID,
			UserID:      ctx.UserID,
			Content:     content,
			Type:        memType,
			Category:    category,
			Tags:        tagsJSON,
			Scope:       scope,
			Provenance:  provenance,
			Importance:  importance,
			State:       targetState,
			CharCount:   len([]rune(content)),
		}
		if scope == "project" {
			m.ProjectID = nilIfEmpty(projectID)
		}
		if err := database.DB.Create(&m).Error; err != nil {
			return nil, err
		}

		// Only active memories create validity records and count toward write usage; pending review memories are handled upon approval
		if targetState == "active" {
			now := time.Now()
			database.DB.Create(&models.MemoryValidity{
				MemoryID:    m.ID,
				WorkspaceID: ctx.WorkspaceID,
				ValidFrom:   now,
				RecordedAt:  now,
			})
			database.DB.Create(&models.UsageRecord{
				WorkspaceID: ctx.WorkspaceID, UserID: ctx.UserID, Metric: "memory_write",
				Quantity: 1, Period: time.Now().Format("2006-01-02"), RecordedAt: time.Now(),
			})
		}

		_ = sourceAgent
		return map[string]interface{}{
			"decision":  decision,
			"memory_id": m.ID,
			"state":     targetState,
			"reason":    reason,
		}, nil
	})

	// 8. hub.save_memory
	r.Register(Tool{
		Name:        "hub.save_memory",
		Description: "Save a memory directly (only when policy allows auto-accept)",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"content":      map[string]interface{}{"type": "string"},
				"type":         map[string]interface{}{"type": "string"},
				"scope":        map[string]interface{}{"type": "string", "description": "workspace (default) or project; for \"project\" the memory is attached to the bound project (bind via project_path or hub.sync_project)"},
				"project_path": projectPathProp,
				"project_id":   map[string]interface{}{"type": "string", "description": "Project id (optional); takes precedence over project_path"},
				"tags":         map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"pinned":       map[string]interface{}{"type": "boolean"},
			},
			Required: []string{"content"},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		content := strArg(args, "content", "")
		if content == "" {
			return nil, fmt.Errorf("content is required")
		}
		memType := strArg(args, "type", "semantic")
		scope := strArg(args, "scope", "workspace")
		pinned := boolArg(args, "pinned", false)

		projectID, err := resolveProjectID(ctx, args)
		if err != nil {
			return nil, err
		}
		if scope == "project" && projectID == "" {
			return nil, errNoProjectBound()
		}
		tagsArr, _ := args["tags"].([]interface{})
		tagsJSON := "[]"
		if len(tagsArr) > 0 {
			tagsJSON = jsonText(tagsArr)
		}

		if services.MemoryQuotaExceeded(ctx.WorkspaceID) {
			return nil, fmt.Errorf("workspace memory count limit reached; cannot save")
		}

		category := "declarative"
		if memType == "skill" {
			category = "procedural"
		}
		m := models.Memory{
			OrgID:       ctx.OrgID,
			WorkspaceID: ctx.WorkspaceID,
			UserID:      ctx.UserID,
			Content:     content,
			Type:        memType,
			Category:    category,
			Tags:        tagsJSON,
			Scope:       scope,
			Provenance:  "agent_extracted",
			Importance:  0.5,
			Pinned:      pinned,
			State:       "active",
			CharCount:   len([]rune(content)),
		}
		if scope == "project" {
			m.ProjectID = nilIfEmpty(projectID)
		}
		if err := database.DB.Create(&m).Error; err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"memory_id": m.ID,
			"status":    "active",
		}, nil
	})

	// 9. hub.update_memory
	r.Register(Tool{
		Name:        "hub.update_memory",
		Description: "Update an existing memory",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"memory_id": map[string]interface{}{"type": "string"},
				"content":   map[string]interface{}{"type": "string"},
			},
			Required: []string{"memory_id", "content"},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		memoryID, _ := args["memory_id"].(string)
		content, _ := args["content"].(string)
		if memoryID == "" || content == "" {
			return nil, fmt.Errorf("memory_id and content are required")
		}
		var m models.Memory
		if err := database.DB.Where("id = ? AND workspace_id = ?", memoryID, ctx.WorkspaceID).First(&m).Error; err != nil {
			return nil, fmt.Errorf("memory not found")
		}
		updates := map[string]interface{}{
			"content":    content,
			"char_count": len([]rune(content)),
			"version":    m.Version + 1,
		}
		database.DB.Model(&m).Updates(updates)
		return map[string]interface{}{
			"memory_id": memoryID,
			"updated":   true,
			"version":   m.Version + 1,
		}, nil
	})

	// 10. hub.archive_memory
	r.Register(Tool{
		Name:        "hub.archive_memory",
		Description: "Archive a memory (mark as archived)",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"memory_id": map[string]interface{}{"type": "string"},
			},
			Required: []string{"memory_id"},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		memoryID, _ := args["memory_id"].(string)
		if memoryID == "" {
			return nil, fmt.Errorf("memory_id is required")
		}
		if err := database.DB.Model(&models.Memory{}).Where("id = ? AND workspace_id = ?", memoryID, ctx.WorkspaceID).Update("state", "archived").Error; err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"memory_id": memoryID,
			"archived":  true,
		}, nil
	})

	// 11. hub.report_action
	r.Register(Tool{
		Name:        "hub.report_action",
		Description: "Report an agent action (for audit and observability)",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"action":  map[string]interface{}{"type": "string"},
				"target":  map[string]interface{}{"type": "string"},
				"summary": map[string]interface{}{"type": "string"},
			},
			Required: []string{"action"},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		action, _ := args["action"].(string)
		target, _ := args["target"].(string)
		summary, _ := args["summary"].(string)

		database.DB.Create(&models.AuditLog{
			WorkspaceID: ctx.WorkspaceID,
			Actor:       ctx.AgentClientID,
			ActorType:   "agent",
			Action:      "agent." + action,
			Target:      target,
			TargetType:  "agent_action",
			Payload:     jsonText(args),
			ClientIP:    ctx.ClientIP,
		})
		return map[string]interface{}{
			"reported": true,
			"id":       uuid.NewString(),
			"action":   action,
			"target":   target,
			"summary":  summary,
		}, nil
	})

	// 12. hub.get_usage_policy
	r.Register(Tool{
		Name:        "hub.get_usage_policy",
		Description: "Get the workspace usage policy and quotas",
		InputSchema: InputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		var ws models.Workspace
		database.DB.First(&ws, "id = ?", ctx.WorkspaceID)

		var memCount int64
		database.DB.Model(&models.Memory{}).Where("workspace_id = ? AND state = 'active'", ctx.WorkspaceID).Count(&memCount)

		today := time.Now().Format("2006-01-02")
		var todayCalls int64
		database.DB.Model(&models.UsageRecord{}).Where("workspace_id = ? AND period = ? AND metric = ?", ctx.WorkspaceID, today, "tool_call").Select("COALESCE(SUM(quantity), 0)").Scan(&todayCalls)
		var monthCalls int64
		month := time.Now().Format("2006-01")
		database.DB.Model(&models.UsageRecord{}).Where("workspace_id = ? AND period = ? AND metric = ?", ctx.WorkspaceID, month, "tool_call").Select("COALESCE(SUM(quantity), 0)").Scan(&monthCalls)

		return map[string]interface{}{
			"quotas": map[string]interface{}{
				"memory_count_max":    ws.QuotaMemoryCount,
				"tool_call_daily_max": ws.QuotaToolCallDaily,
			},
			"usage": map[string]interface{}{
				"memory_count":    memCount,
				"tool_call_today": todayCalls,
				"tool_call_month": monthCalls,
			},
			"policies": map[string]interface{}{
				"high_risk_requires_confirmation": true,
				"max_memory_chars":                2200,
			},
		}, nil
	})

	// 13. hub.get_remaining_quota
	r.Register(Tool{
		Name:        "hub.get_remaining_quota",
		Description: "Get the remaining quota",
		InputSchema: InputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		var ws models.Workspace
		database.DB.First(&ws, "id = ?", ctx.WorkspaceID)

		var memCount int64
		database.DB.Model(&models.Memory{}).Where("workspace_id = ? AND state = 'active'", ctx.WorkspaceID).Count(&memCount)
		today := time.Now().Format("2006-01-02")
		var todayCalls int64
		database.DB.Model(&models.UsageRecord{}).Where("workspace_id = ? AND period = ? AND metric = ?", ctx.WorkspaceID, today, "tool_call").Select("COALESCE(SUM(quantity), 0)").Scan(&todayCalls)

		memRemaining := int64(ws.QuotaMemoryCount) - memCount
		if memRemaining < 0 {
			memRemaining = 0
		}
		callRemaining := int64(ws.QuotaToolCallDaily) - todayCalls
		if callRemaining < 0 {
			callRemaining = 0
		}

		// Next reset time (tomorrow at midnight)
		resetAt := time.Now().Add(24*time.Hour).Format("2006-01-02") + "T00:00:00Z"

		return map[string]interface{}{
			"memory_count_remaining": memRemaining,
			"tool_calls_remaining":   callRemaining,
			"reset_at":               resetAt,
		}, nil
	})

	// 14. hub.get_output_preferences
	r.Register(Tool{
		Name:        "hub.get_output_preferences",
		Description: "Get the user output style preferences",
		InputSchema: InputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		return services.GetOutputPreferencesMap(ctx.UserID)
	})

	// ============================================================
	// P1 tool implementations
	// ============================================================

	// 15. hub.list_connected_tools
	r.Register(Tool{
		Name:        "hub.list_connected_tools",
		Description: "List the workspace's connected external MCP servers and their callable tools",
		InputSchema: InputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		var servers []models.ConnectedMCPServer
		database.DB.Where("workspace_id = ? AND status = ?", ctx.WorkspaceID, "active").Find(&servers)

		type ServerTools struct {
			ServerID  string                   `json:"server_id"`
			Name      string                   `json:"name"`
			Endpoint  string                   `json:"endpoint"`
			Transport string                   `json:"transport"`
			Tools     []map[string]interface{} `json:"tools"`
		}
		result := make([]ServerTools, 0, len(servers))
		for _, s := range servers {
			var tools []map[string]interface{}
			if s.ToolsJSON != "" && s.ToolsJSON != "[]" {
				json.Unmarshal([]byte(s.ToolsJSON), &tools)
			}
			result = append(result, ServerTools{
				ServerID:  s.ID,
				Name:      s.Name,
				Endpoint:  s.Endpoint,
				Transport: s.Transport,
				Tools:     tools,
			})
		}
		return map[string]interface{}{
			"servers": result,
			"count":   len(result),
		}, nil
	})

	// 16. hub.invoke_connected_tool
	r.Register(Tool{
		Name:        "hub.invoke_connected_tool",
		Description: "Invoke a tool on an external MCP server (namespaced as {server}.{tool_name})",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tool":      map[string]interface{}{"type": "string", "description": "Namespaced tool name, e.g. github.create_pull_request"},
				"arguments": map[string]interface{}{"type": "object", "description": "Tool arguments"},
			},
			Required: []string{"tool"},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		toolName, _ := args["tool"].(string)
		if toolName == "" {
			return nil, fmt.Errorf("tool is required")
		}
		toolArgs, _ := args["arguments"].(map[string]interface{})
		if toolArgs == nil {
			toolArgs = map[string]interface{}{}
		}

		parts := strings.SplitN(toolName, ".", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("tool name must be in namespace.format (e.g. github.create_pull_request)")
		}
		namespace := parts[0]
		localToolName := parts[1]

		var server models.ConnectedMCPServer
		if err := database.DB.Where("workspace_id = ? AND name = ? AND status = ?", ctx.WorkspaceID, namespace, "active").First(&server).Error; err != nil {
			return nil, fmt.Errorf("connected server '%s' not found or inactive", namespace)
		}

		var policy models.ToolPolicy
		policyFound := false
		if err := database.DB.Where("workspace_id = ? AND connected_server_id = ? AND tool_name = ?", ctx.WorkspaceID, server.ID, toolName).First(&policy).Error; err == nil {
			policyFound = true
		}

		if policyFound && !policy.Allowed {
			return nil, fmt.Errorf("tool '%s' is blocked by policy", toolName)
		}

		if policyFound && policy.MaxCallsPerDay > 0 {
			today := time.Now().Format("2006-01-02")
			var count int64
			database.DB.Model(&models.ToolInvocationLog{}).
				Where("workspace_id = ? AND tool_name = ? AND status != 'forbidden' AND invoked_at >= ?", ctx.WorkspaceID, toolName, today).
				Count(&count)
			if count >= int64(policy.MaxCallsPerDay) {
				return nil, fmt.Errorf("tool '%s' daily limit (%d) exceeded", toolName, policy.MaxCallsPerDay)
			}
		}

		if policyFound && policy.RequiresConfirmation {
			return map[string]interface{}{
				"requires_confirmation": true,
				"tool":                  toolName,
				"risk_level":            policy.RiskLevel,
				"message":               fmt.Sprintf("Tool '%s' requires user confirmation before execution (risk: %s)", toolName, policy.RiskLevel),
			}, nil
		}

		result, err := invokeUpstreamMCP(server, localToolName, toolArgs)
		if err != nil {
			return map[string]interface{}{
				"success": false,
				"error":   err.Error(),
				"tool":    toolName,
			}, nil
		}

		return map[string]interface{}{
			"success": true,
			"tool":    toolName,
			"result":  result,
		}, nil
	})

	// 17. hub.get_tool_policy
	r.Register(Tool{
		Name:        "hub.get_tool_policy",
		Description: "Get the policy configuration for a tool",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tool": map[string]interface{}{"type": "string", "description": "Namespaced tool name"},
			},
			Required: []string{"tool"},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		toolName, _ := args["tool"].(string)
		if toolName == "" {
			return nil, fmt.Errorf("tool is required")
		}

		var policies []models.ToolPolicy
		database.DB.Where("workspace_id = ? AND tool_name = ?", ctx.WorkspaceID, toolName).Find(&policies)
		if len(policies) == 0 {
			return map[string]interface{}{
				"tool":    toolName,
				"policy":  nil,
				"default": "allowed",
			}, nil
		}
		return map[string]interface{}{
			"tool":     toolName,
			"policies": policies,
		}, nil
	})

	// 18. hub.get_project_context
	r.Register(Tool{
		Name:        "hub.get_project_context",
		Description: "Get the full project context (rules, preferences, and aggregated memories)",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"project_id":   map[string]interface{}{"type": "string", "description": "Project id (optional); takes precedence over project_path"},
				"project_path": projectPathProp,
				"agent_name":   map[string]interface{}{"type": "string", "description": "Target agent name (optional); defaults to the connecting client_type"},
			},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		projectID, err := resolveProjectID(ctx, args)
		if err != nil {
			return nil, err
		}
		if projectID == "" {
			return nil, errNoProjectBound()
		}
		agentName := resolveAgentName(ctx, args)

		var project models.Project
		if err := database.DB.Where("id = ? AND workspace_id = ?", projectID, ctx.WorkspaceID).First(&project).Error; err != nil {
			return nil, fmt.Errorf("project not found")
		}

		// Effective rules: global→project→agent override merge
		effectiveRules := services.ResolveEffectiveRules(ctx.WorkspaceID, &projectID, agentName)

		prefsMap, err := services.GetOutputPreferencesMap(ctx.UserID)
		if err != nil {
			return nil, err
		}

		var mems []models.Memory
		database.DB.Where("workspace_id = ? AND state = 'active' AND (project_id = ? OR project_id IS NULL OR scope = 'workspace')", ctx.WorkspaceID, projectID).
			Order("pinned DESC, importance DESC").Limit(20).Find(&mems)

		skills := availableSkills(ctx, projectID, "all", 10)

		return map[string]interface{}{
			"project":            project,
			"effective_rules":    services.RuleValues(effectiveRules),
			"rules_detail":       effectiveRules,
			"output_preferences": prefsMap,
			"relevant_memories":  mems,
			"skills":             skills,
		}, nil
	})

	// 19. hub.list_skills
	r.Register(Tool{
		Name:        "hub.list_skills",
		Description: "List custom and installed public skills available to the current workspace or bound project",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"project_id":   map[string]interface{}{"type": "string", "description": "Project id (optional); takes precedence over project_path"},
				"project_path": projectPathProp,
				"source":       map[string]interface{}{"type": "string", "enum": []string{"all", "custom", "public"}},
				"limit":        map[string]interface{}{"type": "integer"},
			},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		projectID, err := resolveProjectID(ctx, args)
		if err != nil {
			return nil, err
		}
		source := strArg(args, "source", "all")
		if source != "all" && source != "custom" && source != "public" {
			return nil, fmt.Errorf("source must be all, custom, or public")
		}
		limit := intArg(args, "limit", 50)
		skills := availableSkills(ctx, projectID, source, limit)
		return map[string]interface{}{
			"skills": skills,
			"count":  len(skills),
		}, nil
	})

	// 20. hub.search_skills
	r.Register(Tool{
		Name:        "hub.search_skills",
		Description: "Search custom and installed public skills available to the current workspace or bound project",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query":        map[string]interface{}{"type": "string"},
				"project_id":   map[string]interface{}{"type": "string", "description": "Project id (optional); takes precedence over project_path"},
				"project_path": projectPathProp,
				"source":       map[string]interface{}{"type": "string", "enum": []string{"all", "custom", "public"}},
				"limit":        map[string]interface{}{"type": "integer"},
			},
			Required: []string{"query"},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		query := strArg(args, "query", "")
		if strings.TrimSpace(query) == "" {
			return nil, fmt.Errorf("query is required")
		}
		projectID, err := resolveProjectID(ctx, args)
		if err != nil {
			return nil, err
		}
		source := strArg(args, "source", "all")
		if source != "all" && source != "custom" && source != "public" {
			return nil, fmt.Errorf("source must be all, custom, or public")
		}
		limit := intArg(args, "limit", 10)
		candidates := availableSkills(ctx, projectID, source, 200)
		results := make([]availableSkill, 0, len(candidates))
		for _, s := range candidates {
			haystack := s.Name + "\n" + s.Description + "\n" + strings.Join(s.Tags, " ") + "\n" + s.Content
			s.Relevance = services.Relevance(query, haystack)
			if s.Relevance > 0 {
				results = append(results, s)
			}
		}
		sort.Slice(results, func(i, j int) bool {
			if results[i].Relevance != results[j].Relevance {
				return results[i].Relevance > results[j].Relevance
			}
			if results[i].Pinned != results[j].Pinned {
				return results[i].Pinned
			}
			return results[i].Name < results[j].Name
		})
		if limit <= 0 {
			limit = 10
		}
		if len(results) > limit {
			results = results[:limit]
		}
		return map[string]interface{}{
			"skills": results,
			"count":  len(results),
		}, nil
	})

	// 21. hub.sync_project
	r.Register(Tool{
		Name:        "hub.sync_project",
		Description: "Initialize / bootstrap a project (this is the 'openagent init' entry point): binds your working directory to a project and syncs the local .openagent snapshot — returns team rules, project info, skills, user profile, and key memories as files to write under the project root, plus a managed block for CLAUDE.md/AGENTS.md. When `changed=true`, an `instructions` field tells you exactly how to write these files to disk yourself — MCP cannot write to your filesystem, so you MUST follow `instructions` and persist the `.openagent/` snapshot files and inject the managed block before proceeding. The resolved project binding persists for the rest of this session, so call this once at task start (with your cwd as project_path) before saving project-scoped memories. Pass the previous etag to skip unchanged content.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"project_path":     map[string]interface{}{"type": "string", "description": "Absolute path of your current working directory (the project root), used to resolve the bound project"},
				"git_remote":       map[string]interface{}{"type": "string", "description": "The repo's git remote URL (e.g. output of `git remote get-url origin`), if any. The most reliable way to identify the same project across machines/clones; resolved ahead of project_path."},
				"project_id":       map[string]interface{}{"type": "string", "description": "Project id (optional); takes precedence over project_path"},
				"etag":             map[string]interface{}{"type": "string", "description": "ETag from the previous sync; if content is unchanged the response has changed=false and no files"},
				"register_project": map[string]interface{}{"type": "boolean", "description": "When no project matches project_path, create one bound to that path"},
				"project_name":     map[string]interface{}{"type": "string", "description": "Name for the newly registered project: a short semantic name you derive from the project's content (e.g. its README or purpose) — never a filesystem path. Defaults to the directory basename."},
			},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		projectPath := services.NormalizeRepoPath(strArg(args, "project_path", ""))
		gitRemote := services.NormalizeGitRemote(strArg(args, "git_remote", ""))

		// Resolve project: project_id > (git_remote > repo_path > repo_name fallback) > request-level binding (X-Project-Path)
		var project *models.Project
		if pid, _ := args["project_id"].(string); pid != "" {
			var p models.Project
			if err := database.DB.Where("id = ? AND workspace_id = ?", pid, ctx.WorkspaceID).First(&p).Error; err != nil {
				return nil, fmt.Errorf("project not found")
			}
			project = &p
		} else if projectPath != "" || gitRemote != "" {
			project = services.FindProjectByIdentity(ctx.WorkspaceID, projectPath, gitRemote)
		} else if ctx.ProjectID != "" {
			var p models.Project
			if err := database.DB.Where("id = ? AND workspace_id = ?", ctx.ProjectID, ctx.WorkspaceID).First(&p).Error; err == nil {
				project = &p
			}
		}

		if project == nil && (projectPath != "" || gitRemote != "") {
			if register, _ := args["register_project"].(bool); register {
				name, _ := args["project_name"].(string)
				if name == "" {
					name = services.RepoNameFromPath(projectPath)
				}
				p := models.Project{
					OrgID:       ctx.OrgID,
					WorkspaceID: ctx.WorkspaceID,
					Name:        name,
					Slug:        uniqueProjectSlug(ctx.WorkspaceID, name),
					Status:      "active",
					RepoPath:    projectPath,
					GitRemote:   gitRemote,
					RepoName:    services.RepoNameFromPath(projectPath),
				}
				if err := database.DB.Create(&p).Error; err != nil {
					return nil, fmt.Errorf("failed to register project: %v", err)
				}
				project = &p
			}
		}

		// Backfill identity fields when an existing project is matched but fields are missing/changed (machine switch, directory change, git URL update), keeping display and matching accurate.
		if project != nil {
			if updates := projectIdentityUpdates(project, projectPath, gitRemote); len(updates) > 0 {
				database.DB.Model(project).Updates(updates)
			}
		}

		// Write binding to the current session so subsequent tool calls (e.g. scope=project memory operations) inherit automatically
		if project != nil {
			bindSessionProject(ctx, project.ID)
		}

		bundle, err := services.BuildSyncBundle(ctx.WorkspaceID, ctx.UserID, project)
		if err != nil {
			return nil, fmt.Errorf("failed to build sync bundle: %v", err)
		}

		// Sync observability: record the "person × client × machine" sync for this project (counts as a "touch" regardless of whether changed is true or false).
		if project != nil {
			recordSyncProject(ctx, project, projectPath, bundle.ETag)
		}

		if prev, _ := args["etag"].(string); prev != "" && prev == bundle.ETag {
			return map[string]interface{}{"changed": false, "etag": bundle.ETag}, nil
		}

		result := map[string]interface{}{
			"changed":       true,
			"etag":          bundle.ETag,
			"files":         bundle.Files,
			"managed_block": bundle.ManagedBlock,
			"managed_files": bundle.ManagedFiles,
			"instructions":  services.RenderSyncInstructions(bundle),
		}
		if project != nil {
			result["project"] = map[string]interface{}{
				"id": project.ID, "name": project.Name, "slug": project.Slug,
				"repo_path": project.RepoPath, "git_remote": project.GitRemote, "repo_name": project.RepoName,
			}
		} else {
			result["project"] = nil
			result["hint"] = "no project bound to this path; call again with register_project=true and a semantic project_name (derived from the project's content, not a path) to create one, or bind repo_path in the console"
		}
		return result, nil
	})

	// 22. hub.get_skill
	r.Register(Tool{
		Name:        "hub.get_skill",
		Description: "Get the full details of a single skill by ID (custom memory_id or public install_id)",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"skill_id": map[string]interface{}{"type": "string", "description": "The skill ID (memory_id for custom skills, install_id for public skills)"},
			},
			Required: []string{"skill_id"},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		skillID := strArg(args, "skill_id", "")
		if skillID == "" {
			return nil, fmt.Errorf("skill_id is required")
		}

		// Try custom skill (Memory type=skill)
		var mem models.Memory
		if err := database.DB.Where("id = ? AND workspace_id = ? AND type = ? AND state = ?",
			skillID, ctx.WorkspaceID, "skill", "active").First(&mem).Error; err == nil {
			name, description := skillNameAndDescription(mem.Content)
			return availableSkill{
				Source:      "custom",
				ID:          mem.ID,
				MemoryID:    mem.ID,
				Name:        name,
				Description: description,
				Content:     mem.Content,
				Version:     mem.Version,
				Tags:        parseTagsJSON(mem.Tags),
				Pinned:      mem.Pinned,
			}, nil
		}

		// Try public skill (SkillInstall)
		var install models.SkillInstall
		if err := database.DB.Where("id = ? AND workspace_id = ? AND state = ?",
			skillID, ctx.WorkspaceID, "active").First(&install).Error; err == nil {
			var tpl models.PublicSkillTemplate
			if err := database.DB.Where("id = ? AND status = ?", install.TemplateID, "active").First(&tpl).Error; err == nil {
				content := install.OverrideContent
				if content == "" {
					content = tpl.Content
				}
				return availableSkill{
					Source:      "public",
					ID:          install.ID,
					TemplateID:  tpl.ID,
					InstallID:   install.ID,
					Name:        tpl.Name,
					Description: tpl.Description,
					Content:     content,
					Version:     install.InstalledVersion,
					Tags:        parseTagsJSON(tpl.Tags),
					Pinned:      install.Pinned,
				}, nil
			}
		}

		return nil, fmt.Errorf("skill not found: %s", skillID)
	})

	// 23. hub.get_project_stack
	r.Register(Tool{
		Name:        "hub.get_project_stack",
		Description: "Get the technology stack information for a project (languages, frameworks, dependencies)",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"project_id":   map[string]interface{}{"type": "string", "description": "Project id (optional); takes precedence over project_path"},
				"project_path": projectPathProp,
			},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		projectID, err := resolveProjectID(ctx, args)
		if err != nil {
			return nil, err
		}
		if projectID == "" {
			return nil, errNoProjectBound()
		}
		var project models.Project
		if err := database.DB.Where("id = ? AND workspace_id = ?", projectID, ctx.WorkspaceID).First(&project).Error; err != nil {
			return nil, fmt.Errorf("project not found")
		}
		return map[string]interface{}{
			"project_id":   project.ID,
			"project_name": project.Name,
			"stack":        project.Stack,
		}, nil
	})

	// 24. hub.get_project_structure
	r.Register(Tool{
		Name:        "hub.get_project_structure",
		Description: "Get the directory structure summary for a project",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"project_id":   map[string]interface{}{"type": "string", "description": "Project id (optional); takes precedence over project_path"},
				"project_path": projectPathProp,
			},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		projectID, err := resolveProjectID(ctx, args)
		if err != nil {
			return nil, err
		}
		if projectID == "" {
			return nil, errNoProjectBound()
		}
		var project models.Project
		if err := database.DB.Where("id = ? AND workspace_id = ?", projectID, ctx.WorkspaceID).First(&project).Error; err != nil {
			return nil, fmt.Errorf("project not found")
		}
		return map[string]interface{}{
			"project_id":   project.ID,
			"project_name": project.Name,
			"structure":    project.Structure,
		}, nil
	})

	// 25. hub.update_project_context
	r.Register(Tool{
		Name:        "hub.update_project_context",
		Description: "Update project metadata (description, technology stack, directory structure)",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"project_id":   map[string]interface{}{"type": "string", "description": "Project id (optional); takes precedence over project_path"},
				"project_path": projectPathProp,
				"description":  map[string]interface{}{"type": "string", "description": "Project description"},
				"stack":        map[string]interface{}{"type": "string", "description": "Technology stack (JSON string)"},
				"structure":    map[string]interface{}{"type": "string", "description": "Directory structure (JSON string)"},
			},
		},
	}, func(ctx *Context, args map[string]interface{}) (interface{}, error) {
		projectID, err := resolveProjectID(ctx, args)
		if err != nil {
			return nil, err
		}
		if projectID == "" {
			return nil, errNoProjectBound()
		}
		var project models.Project
		if err := database.DB.Where("id = ? AND workspace_id = ?", projectID, ctx.WorkspaceID).First(&project).Error; err != nil {
			return nil, fmt.Errorf("project not found")
		}

		updates := map[string]interface{}{}
		if v := strPtrArg(args, "description"); v != nil {
			updates["description"] = *v
		}
		if v := strPtrArg(args, "stack"); v != nil {
			updates["stack"] = *v
		}
		if v := strPtrArg(args, "structure"); v != nil {
			updates["structure"] = *v
		}
		if len(updates) == 0 {
			return nil, fmt.Errorf("at least one of description, stack, or structure is required")
		}

		database.DB.Model(&project).Updates(updates)
		database.DB.First(&project, "id = ?", projectID)

		database.DB.Create(&models.AuditLog{
			WorkspaceID: ctx.WorkspaceID,
			Actor:       ctx.UserID,
			ActorType:   "agent",
			Action:      "project.update_context",
			Target:      projectID,
			TargetType:  "project",
			Payload:     jsonText(updates),
			ClientIP:    ctx.ClientIP,
		})

		return map[string]interface{}{
			"project_id":  project.ID,
			"name":        project.Name,
			"description": project.Description,
			"stack":       project.Stack,
			"structure":   project.Structure,
			"updated":     true,
		}, nil
	})
}

// projectPathProp is the common schema snippet for the project_path tool input parameter.
var projectPathProp = map[string]interface{}{
	"type":        "string",
	"description": "Absolute path of your current working directory, used to resolve the bound project (optional when a session or X-Project-Path binding exists)",
}

// resolveProjectID uniformly parses the project binding for a tool call, priority:
// explicit project_id param > explicit project_path param > session/request-level binding (ctx.ProjectID).
// When project_path is given but matches no project, an error is returned to guide the agent to register first;
// when none of the three are present, an empty string is returned (the caller decides whether it is required).
func resolveProjectID(ctx *Context, args map[string]interface{}) (string, error) {
	if pid, _ := args["project_id"].(string); pid != "" {
		var p models.Project
		if err := database.DB.Where("id = ? AND workspace_id = ?", pid, ctx.WorkspaceID).First(&p).Error; err != nil {
			return "", fmt.Errorf("project not found: %s", pid)
		}
		return p.ID, nil
	}
	path, _ := args["project_path"].(string)
	gitRemote, _ := args["git_remote"].(string)
	if path != "" || gitRemote != "" {
		if p := services.FindProjectByIdentity(ctx.WorkspaceID, path, gitRemote); p != nil {
			return p.ID, nil
		}
		return "", fmt.Errorf("no project is bound to %q: call hub.sync_project with project_path=<your working directory>, register_project=true and a semantic project_name first; the binding then persists for this session", path)
	}
	return ctx.ProjectID, nil
}

// projectIdentityUpdates computes identity fields to backfill for a matched project: fill in missing git_remote/repo_name,
// and update repo_path to the current value (keeps last-seen meaningful when the same project switches machines/directories). An empty map means no updates needed.
func projectIdentityUpdates(p *models.Project, repoPath, gitRemote string) map[string]interface{} {
	updates := map[string]interface{}{}
	if gitRemote != "" && p.GitRemote != gitRemote {
		updates["git_remote"] = gitRemote
		p.GitRemote = gitRemote
	}
	if repoPath != "" {
		if p.RepoPath != repoPath {
			updates["repo_path"] = repoPath
			p.RepoPath = repoPath
		}
		if name := services.RepoNameFromPath(repoPath); name != "" && p.RepoName != name {
			updates["repo_name"] = name
			p.RepoName = name
		}
	}
	return updates
}

// errNoProjectBound is the unified error when scope=project but no project can be resolved.
func errNoProjectBound() error {
	return fmt.Errorf("scope is \"project\" but no project is bound: pass project_path=<your working directory>, or call hub.sync_project with register_project=true and a semantic project_name to create and bind one for this session")
}

// bindSessionProject writes the resolved project into the current MCP session so subsequent calls in the same session inherit the binding automatically.
func bindSessionProject(ctx *Context, projectID string) {
	if projectID == "" {
		return
	}
	ctx.ProjectID = projectID
	if ctx.MCPSessionID != "" {
		database.DB.Model(&models.MCPSession{}).Where("id = ?", ctx.MCPSessionID).Update("project_id", projectID)
	}
}

// recordSyncProject records a project sync via sidecar for multi-person multi-client sync observability.
// Identity key = (project_id, user_id, agent_client_id, repo_path): repeated syncs with the same identity only update
// etag/synced_at and increment sync_count; switching machines (different repo_path) or clients (different agent_client_id) creates a new row.
// Sidecar audit only; any failure must not affect the main sync flow.
func recordSyncProject(ctx *Context, project *models.Project, repoPath, etag string) {
	client, clientName := "unknown", "Unknown Agent"
	if ctx.AgentClientID != "" {
		var ac models.AgentClient
		if err := database.DB.Select("client_type", "client_name").
			First(&ac, "id = ?", ctx.AgentClientID).Error; err == nil {
			if ac.ClientType != "" {
				client = ac.ClientType
			}
			if ac.ClientName != "" {
				clientName = ac.ClientName
			}
		}
	}

	now := time.Now()
	var existing models.SyncRecord
	err := database.DB.Where(
		"project_id = ? AND user_id = ? AND agent_client_id = ? AND repo_path = ?",
		project.ID, ctx.UserID, ctx.AgentClientID, repoPath,
	).First(&existing).Error
	if err == nil {
		database.DB.Model(&existing).Updates(map[string]interface{}{
			"client":      client,
			"client_name": clientName,
			"etag":        etag,
			"sync_count":  existing.SyncCount + 1,
			"synced_at":   now,
		})
		return
	}
	database.DB.Create(&models.SyncRecord{
		OrgID:         ctx.OrgID,
		WorkspaceID:   ctx.WorkspaceID,
		ProjectID:     project.ID,
		UserID:        ctx.UserID,
		AgentClientID: ctx.AgentClientID,
		Client:        client,
		ClientName:    clientName,
		RepoPath:      repoPath,
		ETag:          etag,
		SyncCount:     1,
		SyncedAt:      now,
	})
}

// uniqueProjectSlug generates a non-conflicting project slug within a workspace (appends -2, -3… on conflict).
func uniqueProjectSlug(workspaceID, name string) string {
	base := slugify(name)
	slug := base
	for i := 2; ; i++ {
		var n int64
		database.DB.Model(&models.Project{}).Where("workspace_id = ? AND slug = ?", workspaceID, slug).Count(&n)
		if n == 0 {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

// nilIfEmpty converts an empty string to nil, used for nullable foreign key columns.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// slugify generates a URL-friendly slug (lowercase alphanumeric + hyphens).
func slugify(s string) string {
	var b strings.Builder
	prevDash := true // suppress leading hyphens
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if out == "" {
		out = "project"
	}
	return out
}

// invokeUpstreamMCP calls an external MCP Server's tool via the circuit breaker. Each server has its own breaker;
// consecutive transport-layer failures trip the circuit (fast fail), with half-open probing after cooldown.
func invokeUpstreamMCP(server models.ConnectedMCPServer, toolName string, args map[string]interface{}) (interface{}, error) {
	cb := breakerFor(server.ID)
	if !cb.Allow() {
		return nil, fmt.Errorf("%w: server %q has failed repeatedly and the circuit is open; retry later", ErrCircuitOpen, server.Name)
	}
	result, tripped, err := doUpstreamCall(server, toolName, args)
	if tripped {
		cb.onFailure()
	} else {
		// 4xx / upstream business error means the peer is healthy; treat as success to reset the breaker count
		cb.onSuccess()
	}
	return result, err
}

// doUpstreamCall performs the actual upstream call. The tripped return value indicates whether the error should count toward breaker failures:
// only "transport errors / 5xx" count; 4xx and upstream JSON-RPC errors do not (the peer is healthy).
func doUpstreamCall(server models.ConnectedMCPServer, toolName string, args map[string]interface{}) (interface{}, bool, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      uuid.NewString(),
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": args,
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, false, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", server.Endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, false, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Credentials are stored encrypted in the DB; decrypt before calling (backward compatible with legacy plaintext rows: return as-is on decryption failure)
	authPlain := database.DecryptAES(server.AuthConfig)
	if server.AuthType == "bearer" && authPlain != "" {
		var authCfg map[string]string
		if json.Unmarshal([]byte(authPlain), &authCfg) == nil {
			if token, ok := authCfg["token"]; ok {
				req.Header.Set("Authorization", "Bearer "+token)
			}
		}
	} else if server.AuthType == "api_key" && authPlain != "" {
		var authCfg map[string]string
		if json.Unmarshal([]byte(authPlain), &authCfg) == nil {
			if key, ok := authCfg["key"]; ok {
				req.Header.Set("Authorization", "Bearer "+key)
			}
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, fmt.Errorf("read upstream response: %w", err)
	}

	if resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("upstream returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	if resp.StatusCode >= 400 {
		return nil, false, fmt.Errorf("upstream returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var jsonResp map[string]interface{}
	if err := json.Unmarshal(respBody, &jsonResp); err != nil {
		return string(respBody), false, nil
	}

	if jsonErr, ok := jsonResp["error"]; ok {
		return nil, false, fmt.Errorf("upstream MCP error: %v", jsonErr)
	}

	if result, ok := jsonResp["result"]; ok {
		return result, false, nil
	}

	return jsonResp, false, nil
}
