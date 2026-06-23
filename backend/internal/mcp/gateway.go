package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openagenthub/backend/internal/auth"
	"github.com/openagenthub/backend/internal/config"
	"github.com/openagenthub/backend/internal/database"
	"github.com/openagenthub/backend/internal/models"
	"github.com/openagenthub/backend/internal/services"
	"golang.org/x/crypto/bcrypt"
)

// Gateway MCP Gateway
type Gateway struct {
	cfg      *config.Config
	registry *ToolRegistry

	// Legacy SSE support: session_id -> channel for sending responses
	sseMu       sync.RWMutex
	sseSessions map[string]chan []byte
}

func NewGateway(cfg *config.Config) *Gateway {
	registry := NewToolRegistry()
	RegisterP0Tools(registry)
	return &Gateway{cfg: cfg, registry: registry, sseSessions: make(map[string]chan []byte)}
}

// AuthenticateAndContext parses the Authorization header and populates ctx
func (g *Gateway) AuthenticateAndContext(c *gin.Context) (*Context, error) {
	authHeader := c.GetHeader("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, &MCPError{Code: ErrCodeUnauthorized, Message: "missing bearer token"}
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Prefer JWT (SaaS Web users)
	if claims, err := g.tryJWT(token); err == nil && claims != nil {
		ctx, err := g.ctxFromJWT(claims, c)
		if err == nil && ctx != nil {
			resolveProjectBinding(ctx, c)
		}
		return ctx, err
	}

	// Otherwise try API Key (MCP Token)
	if ctx, err := g.tryAPIKey(token, c); err == nil && ctx != nil {
		resolveProjectBinding(ctx, c)
		return ctx, nil
	}

	return nil, &MCPError{Code: ErrCodeUnauthorized, Message: "invalid token"}
}

// resolveProjectBinding binds the project to the Context based on the X-Project-Path header.
// Silently skips when no project is matched (the tool layer will provide a hint).
func resolveProjectBinding(ctx *Context, c *gin.Context) {
	projectPath := c.GetHeader("X-Project-Path")
	if projectPath == "" {
		return
	}
	if p := services.FindProjectByPath(ctx.WorkspaceID, projectPath); p != nil {
		ctx.ProjectID = p.ID
	}
}

func (g *Gateway) tryJWT(tokenString string) (interface{}, error) {
	claims, err := auth.ParseJWT(g.cfg, tokenString)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"user_id":      claims.UserID,
		"username":     claims.Username,
		"workspace_id": claims.WorkspaceID,
		"org_id":       claims.OrgID,
		"role":         claims.Role,
		"scopes":       claims.Scopes,
		"exp":          claims.ExpiresAt.Unix(),
	}, nil
}

func (g *Gateway) ctxFromJWT(claims interface{}, c *gin.Context) (*Context, error) {
	m, ok := claims.(map[string]interface{})
	if !ok {
		return nil, &MCPError{Code: ErrCodeUnauthorized, Message: "invalid claims"}
	}
	workspaceID, _ := m["workspace_id"].(string)
	userID, _ := m["user_id"].(string)
	orgID, _ := m["org_id"].(string)
	role, _ := m["role"].(string)
	if workspaceID == "" || userID == "" {
		return nil, &MCPError{Code: ErrCodeUnauthorized, Message: "missing workspace_id or user_id"}
	}
	ctx := &Context{
		WorkspaceID: workspaceID,
		UserID:      userID,
		OrgID:       orgID,
		Role:        role,
		ClientIP:    c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
	}
	return ctx, nil
}

func (g *Gateway) tryAPIKey(token string, c *gin.Context) (*Context, error) {
	// Only accept tokens starting with pat_
	if !strings.HasPrefix(token, "pat_") {
		return nil, nil
	}

	// Compute token hash index: bcrypt verification requires iterating all keys
	// Optimization: use sha256 prefix index for fast filtering
	h := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(h[:])

	// First try a sha256 hash lookup (if indexed)
	_ = tokenHash

	// Since the hash is bcrypt, we must iterate all non-revoked keys for comparison
	// As an optimization, filter by prefix first
	prefix := token[:11]
	var keys []models.APIKey
	if err := database.DB.Where("prefix = ? AND revoked_at IS NULL", prefix).Find(&keys).Error; err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, nil
	}

	for _, k := range keys {
		if err := bcrypt.CompareHashAndPassword([]byte(k.Hash), []byte(token)); err == nil {
			// Found a matching key
			if k.ExpiresAt != nil && k.ExpiresAt.Before(time.Now()) {
				return nil, &MCPError{Code: ErrCodeUnauthorized, Message: "token expired"}
			}
			// Load workspace and user info
			var ws models.Workspace
			database.DB.First(&ws, "id = ?", k.WorkspaceID)
			// Default user: workspace owner
			var ownerMember models.WorkspaceMember
			database.DB.Where("workspace_id = ? AND role = ? AND status = ?", k.WorkspaceID, "owner", "active").First(&ownerMember)
			ctx := &Context{
				WorkspaceID: k.WorkspaceID,
				UserID:      ownerMember.UserID,
				OrgID:       ws.OrgID,
				Role:        "service",
				ClientIP:    c.ClientIP(),
				UserAgent:   c.GetHeader("User-Agent"),
			}
			// Update last_used_at
			now := time.Now()
			database.DB.Model(&k).Update("last_used_at", &now)
			return ctx, nil
		}
	}
	return nil, nil
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int
	Message string
}

func (e *MCPError) Error() string {
	return e.Message
}

// HandleHTTP handles HTTP POST (JSON-RPC over HTTP)
// Supports both Streamable HTTP (direct JSON response) and Legacy SSE (response sent via SSE channel)
func (g *Gateway) HandleHTTP(c *gin.Context) {
	ctx, err := g.AuthenticateAndContext(c)
	if err != nil {
		if mcpErr, ok := err.(*MCPError); ok {
			c.JSON(http.StatusUnauthorized, Response{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   &Error{Code: mcpErr.Code, Message: mcpErr.Message},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{
			JSONRPC: "2.0",
			Error:   &Error{Code: ErrCodeInternalError, Message: err.Error()},
		})
		return
	}

	// Read body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			JSONRPC: "2.0",
			Error:   &Error{Code: ErrCodeParseError, Message: "read body: " + err.Error()},
		})
		return
	}

	// Parse JSON-RPC
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			JSONRPC: "2.0",
			Error:   &Error{Code: ErrCodeParseError, Message: "parse: " + err.Error()},
		})
		return
	}

	// Record session
	sessionID := c.GetHeader("Mcp-Session-Id")
	if sessionID == "" {
		sessionID = c.Query("session_id")
	}
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	g.ensureSession(ctx, sessionID, c)
	// Send back the session ID so Streamable HTTP clients can echo it in subsequent requests,
	// enabling session-level project bindings (mcp_sessions.project_id).
	c.Header("Mcp-Session-Id", sessionID)

	// Route method
	resp := g.handleMethod(ctx, &req, c)
	if resp.ID == nil {
		resp.ID = req.ID
	}

	// Check for an active SSE session (Legacy SSE mode)
	// If session_id was passed via query param and there is an active SSE channel, use SSE
	if c.Query("session_id") != "" {
		g.sseMu.RLock()
		respChan, hasSSE := g.sseSessions[sessionID]
		g.sseMu.RUnlock()
		if hasSSE {
			respBytes, _ := json.Marshal(resp)
			select {
			case respChan <- respBytes:
				c.Status(http.StatusAccepted)
				return
			default:
				// channel full, fallback to direct response
			}
		}
	}

	// Streamable HTTP mode: return JSON directly
	c.JSON(http.StatusOK, resp)
}

// HandleSSE handles SSE connections (GET /mcp)
// Supports both Streamable HTTP SSE and Legacy SSE clients (e.g. Cline, Cursor)
func (g *Gateway) HandleSSE(c *gin.Context) {
	_, err := g.AuthenticateAndContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, Response{
			JSONRPC: "2.0",
			Error:   &Error{Code: ErrCodeUnauthorized, Message: "authentication failed"},
		})
		return
	}

	sessionID := c.GetHeader("Mcp-Session-Id")
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	// Register SSE session (for Legacy SSE POST relay)
	respChan := make(chan []byte, 64)
	g.sseMu.Lock()
	g.sseSessions[sessionID] = respChan
	g.sseMu.Unlock()

	defer func() {
		g.sseMu.Lock()
		delete(g.sseSessions, sessionID)
		g.sseMu.Unlock()
		close(respChan)
	}()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Mcp-Session-Id", sessionID)

	// Send Legacy SSE endpoint event (plain URL, not JSON)
	endpointURL := fmt.Sprintf("/mcp?session_id=%s", sessionID)
	c.Writer.WriteString(fmt.Sprintf("event: endpoint\ndata: %s\n\n", endpointURL))
	c.Writer.Flush()

	// Keep the connection alive, push POST responses via SSE
	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-respChan:
			if !ok {
				return false
			}
			c.Writer.WriteString(fmt.Sprintf("event: message\ndata: %s\n\n", string(msg)))
			c.Writer.Flush()
			return true
		case <-c.Request.Context().Done():
			return false
		case <-time.After(30 * time.Minute):
			return false
		}
	})
}

// HandleLegacySSE handles Legacy SSE connections (GET /sse)
// This is the transport protocol used by older MCP clients (e.g. Cursor, Claude Desktop)
func (g *Gateway) HandleLegacySSE(c *gin.Context) {
	_, err := g.AuthenticateAndContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, Response{
			JSONRPC: "2.0",
			Error:   &Error{Code: ErrCodeUnauthorized, Message: "authentication failed"},
		})
		return
	}

	sessionID := uuid.NewString()

	// Create response channel
	respChan := make(chan []byte, 64)
	g.sseMu.Lock()
	g.sseSessions[sessionID] = respChan
	g.sseMu.Unlock()

	// Clean up on connection close
	defer func() {
		g.sseMu.Lock()
		delete(g.sseSessions, sessionID)
		g.sseMu.Unlock()
		close(respChan)
	}()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Send the endpoint event to tell the client the POST message URL
	endpointURL := fmt.Sprintf("/message?session_id=%s", sessionID)
	c.Writer.WriteString(fmt.Sprintf("event: endpoint\ndata: %s\n\n", endpointURL))
	c.Writer.Flush()

	// Keep the connection alive, push responses to the client via SSE
	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-respChan:
			if !ok {
				return false
			}
			c.Writer.WriteString(fmt.Sprintf("event: message\ndata: %s\n\n", string(msg)))
			c.Writer.Flush()
			return true
		case <-c.Request.Context().Done():
			return false
		case <-time.After(30 * time.Minute):
			// Timeout close
			return false
		}
	})
}

// HandleLegacyMessage handles Legacy SSE messages (POST /message)
// Clients send JSON-RPC requests through this endpoint; responses are returned via the SSE stream
func (g *Gateway) HandleLegacyMessage(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session_id"})
		return
	}

	g.sseMu.RLock()
	respChan, ok := g.sseSessions[sessionID]
	g.sseMu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	// Set session header so AuthenticateAndContext can use it
	c.Request.Header.Set("Mcp-Session-Id", sessionID)

	// Read and process the JSON-RPC request (reuses HandleHTTP logic)
	ctx, err := g.AuthenticateAndContext(c)
	if err != nil {
		if mcpErr, ok := err.(*MCPError); ok {
			c.JSON(http.StatusUnauthorized, Response{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   &Error{Code: mcpErr.Code, Message: mcpErr.Message},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{
			JSONRPC: "2.0",
			Error:   &Error{Code: ErrCodeInternalError, Message: err.Error()},
		})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			JSONRPC: "2.0",
			Error:   &Error{Code: ErrCodeParseError, Message: "failed to read body"},
		})
		return
	}

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			JSONRPC: "2.0",
			Error:   &Error{Code: ErrCodeParseError, Message: "invalid JSON"},
		})
		return
	}

	g.ensureSession(ctx, sessionID, c)

	resp := g.handleMethod(ctx, &req, c)
	if resp.ID == nil {
		resp.ID = req.ID
	}

	// Send the response to the SSE channel
	respBytes, _ := json.Marshal(resp)
	select {
	case respChan <- respBytes:
		c.Status(http.StatusAccepted)
	default:
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "channel full"})
	}
}

func (g *Gateway) ensureSession(ctx *Context, sessionID string, c *gin.Context) {
	if sessionID == "" {
		return
	}
	hash := sha256.Sum256([]byte(c.GetHeader("Authorization")))
	tokenHash := hex.EncodeToString(hash[:])
	now := time.Now()

	// upsert session
	var existing models.MCPSession
	if err := database.DB.Where("id = ?", sessionID).First(&existing).Error; err == nil {
		ctx.MCPSessionID = sessionID
		ctx.AgentClientID = existing.AgentClientID
		// Session-level project binding takes priority over X-Project-Path header
		// (session binding comes from agent's explicitly synced cwd)
		if existing.ProjectID != "" {
			ctx.ProjectID = existing.ProjectID
		}
		database.DB.Model(&existing).Updates(map[string]interface{}{
			"last_activity_at": &now,
			"status":           "active",
		})
	} else {
		// agent_client_id: try to detect from user_agent
		agentClientID := g.detectAgentClient(ctx, c.GetHeader("User-Agent"))
		ctx.AgentClientID = agentClientID
		ctx.MCPSessionID = sessionID

		database.DB.Create(&models.MCPSession{
			BaseModel:       models.BaseModel{ID: sessionID},
			WorkspaceID:     ctx.WorkspaceID,
			UserID:          ctx.UserID,
			AgentClientID:   agentClientID,
			AccessTokenHash: tokenHash,
			Scopes:          "[\"read\",\"write\"]",
			Status:          "active",
			StartedAt:       now,
			LastActivityAt:  &now,
			ClientIP:        c.ClientIP(),
			UserAgent:       c.GetHeader("User-Agent"),
		})
	}
}

func (g *Gateway) detectAgentClient(ctx *Context, ua string) string {
	uaLower := strings.ToLower(ua)
	clientType := "unknown"
	clientName := "Unknown Agent"
	switch {
	case strings.Contains(uaLower, "cursor"):
		clientType = "cursor"
		clientName = "Cursor"
	case strings.Contains(uaLower, "claude-code"):
		clientType = "claude-code"
		clientName = "Claude Code"
	case strings.Contains(uaLower, "windsurf"):
		clientType = "windsurf"
		clientName = "Windsurf"
	case strings.Contains(uaLower, "opencode"):
		clientType = "opencode"
		clientName = "OpenCode"
	case strings.Contains(uaLower, "copilot"):
		clientType = "copilot"
		clientName = "GitHub Copilot"
	}

	now := time.Now()
	var client models.AgentClient
	// Agent Profile is user-private data, decoupled from workspace:
	// identity is (user_id, client_type); workspace_id only records the "most recently active workspace";
	// deleting a workspace does not lose the agent profile.
	err := database.DB.Where("user_id = ? AND client_type = ?", ctx.UserID, clientType).First(&client).Error
	if err != nil {
		client = models.AgentClient{
			WorkspaceID: ctx.WorkspaceID,
			UserID:      ctx.UserID,
			ClientType:  clientType,
			ClientName:  clientName,
			Status:      "active",
			FirstSeenAt: now,
			LastSeenAt:  &now,
		}
		database.DB.Create(&client)
	} else {
		database.DB.Model(&client).Updates(map[string]interface{}{
			"last_seen_at": &now,
			"workspace_id": ctx.WorkspaceID,
		})
	}
	return client.ID
}

func (g *Gateway) handleMethod(ctx *Context, req *Request, c *gin.Context) Response {
	switch req.Method {
	case "initialize":
		return g.handleInitialize(ctx, req, c)
	case "tools/list":
		return g.handleToolsList(ctx, req, c)
	case "tools/call":
		return g.handleToolsCall(ctx, req, c)
	case "resources/list":
		return g.handleResourcesList(ctx, req, c)
	case "resources/read":
		return g.handleResourcesRead(ctx, req, c)
	case "prompts/list":
		return g.handlePromptsList(ctx, req, c)
	case "prompts/get":
		return g.handlePromptsGet(ctx, req, c)
	case "ping":
		return Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}
	default:
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: ErrCodeMethodNotFound, Message: "method not found: " + req.Method},
		}
	}
}

func (g *Gateway) handleInitialize(ctx *Context, req *Request, c *gin.Context) Response {
	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: InitializeResult{
			ProtocolVersion: "2025-11-25",
			ServerInfo: ServerInfo{
				Name:    "open-agent-hub",
				Version: "0.1.0",
			},
			Capabilities: ServerCapabilities{
				Tools:     &ToolsCapability{ListChanged: false},
				Resources: &ResourcesCapability{Subscribe: false, ListChanged: false},
				Prompts:   &PromptsCapability{ListChanged: false},
			},
			Instructions: `Open Agent Hub — unified rules, memory, skills, and tool routing for AI agents.

STARTUP WORKFLOW (do this first, or whenever the user says "init", "openagent init", or "bootstrap"):
1. Call hub.sync_project with project_path = your current working directory (absolute path). If it says no project is bound, call again with register_project=true and a semantic project_name.
2. When the response has changed=true, it includes an "instructions" field — you MUST follow those instructions to write the .openagent/ snapshot files to disk yourself (MCP cannot write files for you), inject the managed_block into CLAUDE.md/AGENTS.md, and persist the etag.
3. After writing files, read .openagent/rules.md, .openagent/project.md (if present), .openagent/local/profile.md, and .openagent/local/memories.md to load your context.
4. Call hub.get_global_rules and hub.get_project_rules for full rule sets.
5. Call hub.get_output_preferences for user formatting preferences.
6. Call hub.search_memory to find relevant past knowledge.

KEY RULES:
- Never edit .openagent/ files directly; use hub.propose_memory to persist new knowledge.
- MCP Tokens are workspace-scoped; data isolation is per-workspace.
- Call hub.sync_project once per session; subsequent calls can pass the etag to skip unchanged content.`,
		},
	}
}

func (g *Gateway) handleToolsList(ctx *Context, req *Request, c *gin.Context) Response {
	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": g.registry.List(),
		},
	}
}

func (g *Gateway) handleToolsCall(ctx *Context, req *Request, c *gin.Context) Response {
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: ErrCodeInvalidParams, Message: "params must be object"},
		}
	}
	name, _ := params["name"].(string)
	args, _ := params["arguments"].(map[string]interface{})

	start := time.Now()
	_, handler, ok := g.registry.Get(name)
	if !ok {
		g.logInvocation(ctx, name, args, "error", ErrCodeToolNotFound, "tool not found", time.Since(start).Milliseconds(), c)
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: ErrCodeToolNotFound, Message: "tool not found: " + name},
		}
	}

	// Tool Policy full-field validation + quota enforcement + confirmation
	isHub := strings.HasPrefix(name, "hub.")
	confirmed := argConfirmed(args)
	delete(args, "__confirm") // not passed through to handler / upstream
	verdict := services.CheckToolCall(ctx.WorkspaceID, ctx.UserID, name, isHub, g.cfg.EnableConfirmation, confirmed)
	switch verdict.Decision {
	case services.DecisionDeny:
		code := ErrCodeForbidden
		switch verdict.Code {
		case "quota_exceeded", "tool_quota_exceeded", "tool_user_quota_exceeded":
			code = ErrCodeRateLimited
		}
		g.logInvocation(ctx, name, args, "forbidden", code, verdict.Reason, time.Since(start).Milliseconds(), c)
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: code, Message: verdict.Reason, Data: verdict},
		}
	case services.DecisionConfirm:
		g.logInvocation(ctx, name, args, "needs_confirmation", ErrCodeToolRequiresConfirm, verdict.Reason, time.Since(start).Milliseconds(), c)
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: ErrCodeToolRequiresConfirm, Message: verdict.Reason, Data: verdict},
		}
	}

	result, err := handler(ctx, args)
	if err != nil {
		g.logInvocation(ctx, name, args, "error", ErrCodeInternalError, err.Error(), time.Since(start).Milliseconds(), c)
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: ErrCodeInternalError, Message: err.Error()},
		}
	}

	g.logInvocation(ctx, name, args, "ok", 0, "", time.Since(start).Milliseconds(), c)
	// Record usage
	database.DB.Create(&models.UsageRecord{
		WorkspaceID: ctx.WorkspaceID, UserID: ctx.UserID, Metric: "tool_call",
		Quantity: 1, Period: time.Now().Format("2006-01-02"), RecordedAt: time.Now(),
	})

	// Return standard MCP ToolCallResult
	text, _ := json.Marshal(result)
	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: ToolCallResult{
			Content: []ContentItem{{Type: "text", Text: string(text)}},
			IsError: false,
		},
	}
}

// argConfirmed reads the secondary confirmation flag __confirm (bool) from the call arguments.
func argConfirmed(args map[string]interface{}) bool {
	if v, ok := args["__confirm"]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func (g *Gateway) logInvocation(ctx *Context, toolName string, input map[string]interface{}, status string, errCode int, errMsg string, latencyMs int64, c *gin.Context) {
	inputJSON, _ := json.Marshal(input)
	var connectedServerID *string
	// Namespace format: {server}.{tool}
	if parts := strings.SplitN(toolName, ".", 2); len(parts) == 2 && !strings.HasPrefix(toolName, "hub.") {
		var srv models.ConnectedMCPServer
		if database.DB.Where("workspace_id = ? AND name = ?", ctx.WorkspaceID, parts[0]).First(&srv).Error == nil {
			connectedServerID = &srv.ID
		}
	}
	acID := ctx.AgentClientID
	sessID := ctx.MCPSessionID
	database.DB.Create(&models.ToolInvocationLog{
		WorkspaceID:       ctx.WorkspaceID,
		UserID:            ctx.UserID,
		AgentClientID:     acID,
		MCPSessionID:      sessID,
		ToolName:          toolName,
		ConnectedServerID: connectedServerID,
		InputJSON:         string(inputJSON),
		Status:            status,
		ErrorCode:         intToStr(errCode),
		ErrorMessage:      errMsg,
		LatencyMs:         int(latencyMs),
		InvokedAt:         time.Now(),
	})
}

func intToStr(i int) string {
	if i == 0 {
		return ""
	}
	return strconv.Itoa(i)
}

func (g *Gateway) handleResourcesList(ctx *Context, req *Request, c *gin.Context) Response {
	resources := []Resource{
		{
			URI:         "hub://workspace/" + ctx.WorkspaceID + "/rules/global",
			Name:        "Workspace Global Rules",
			Description: "Workspace global rules (with ETag caching)",
			MimeType:    "application/json",
		},
		{
			URI:         "hub://workspace/" + ctx.WorkspaceID + "/tool-policies",
			Name:        "Workspace Tool Policies",
			Description: "Workspace tool policy",
			MimeType:    "application/json",
		},
		{
			URI:         "hub://workspace/" + ctx.WorkspaceID + "/skills",
			Name:        "Workspace Skills",
			Description: "Workspace available skills",
			MimeType:    "application/json",
		},
		{
			URI:         "hub://workspace/" + ctx.WorkspaceID + "/memory/snapshot/latest",
			Name:        "Memory Snapshot",
			Description: "Memory snapshot (frozen view)",
			MimeType:    "application/json",
		},
	}
	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"resources": resources},
	}
}

func (g *Gateway) handleResourcesRead(ctx *Context, req *Request, c *gin.Context) Response {
	params, _ := req.Params.(map[string]interface{})
	uri, _ := params["uri"].(string)

	var result map[string]interface{}
	switch {
	case strings.HasSuffix(uri, "/rules/global"):
		var rules []models.Rule
		database.DB.Where("workspace_id = ? AND scope = ?", ctx.WorkspaceID, "workspace").Find(&rules)
		result = map[string]interface{}{
			"rules":   rules,
			"version": time.Now().UTC().Format(time.RFC3339),
			"etag":    computeETag(ctx.WorkspaceID, "global"),
		}
	case strings.HasSuffix(uri, "/tool-policies"):
		var policies []models.ToolPolicy
		database.DB.Where("workspace_id = ?", ctx.WorkspaceID).Find(&policies)
		result = map[string]interface{}{"policies": policies}
	case strings.HasSuffix(uri, "/skills"):
		var skills []models.Memory
		database.DB.Where("workspace_id = ? AND type = 'skill' AND state IN ('active','stale')", ctx.WorkspaceID).Find(&skills)
		result = map[string]interface{}{"skills": skills}
	case strings.HasSuffix(uri, "/memory/snapshot/latest"):
		var mems []models.Memory
		database.DB.Where("workspace_id = ? AND state = 'active'", ctx.WorkspaceID).Order("pinned DESC, importance DESC").Find(&mems)
		result = map[string]interface{}{
			"memory_count": len(mems),
			"items":        mems,
			"frozen_at":    time.Now().UTC().Format(time.RFC3339),
		}
	default:
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: ErrCodeMethodNotFound, Message: "resource not found: " + uri},
		}
	}

	c.Header("ETag", computeETag(uri, jsonText(result)))
	c.Header("Cache-Control", "max-age=300, must-revalidate")

	text, _ := json.Marshal(result)
	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"contents": []map[string]interface{}{{
				"uri":      uri,
				"mimeType": "application/json",
				"text":     string(text),
			}},
		},
	}
}

func (g *Gateway) handlePromptsList(ctx *Context, req *Request, c *gin.Context) Response {
	prompts := []Prompt{
		{
			Name:        "open_agent_hub_project_bootstrap",
			Description: "Project bootstrap: load rules/preferences/memories before starting work",
		},
		{
			Name:        "open_agent_hub_code_review",
			Description: "Code review: run PR review using skills + rules",
		},
		{
			Name:        "open_agent_hub_memory_review",
			Description: "Memory review: let the user review agent-proposed memories",
		},
		{
			Name:        "open_agent_hub_vibecoding_plan",
			Description: "VibeCoding planning: turn natural-language requirements into technical tasks",
		},
	}
	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"prompts": prompts},
	}
}

func (g *Gateway) handlePromptsGet(ctx *Context, req *Request, c *gin.Context) Response {
	params, _ := req.Params.(map[string]interface{})
	name, _ := params["name"].(string)

	var content string
	switch name {
	case "open_agent_hub_project_bootstrap":
		content = `You are a development Agent that follows the Open Agent Hub Workspace rules.

Please start with the following steps:
1. Call hub.sync_project to bind the current project: pass your working directory absolute path as project_path;
   if it reports not bound, retry with register_project=true and project_name (a semantic name based on
   project content, not the path). The binding lasts for this session; subsequent scope=project operations
   will automatically attach to this project. If changed=true is returned, the instructions field in the
   response tells you how to write the returned files to local disk (the MCP server cannot write files
   directly) — you must follow the instructions to write the .openagent/ snapshot files to the project root,
   and inject managed_block into CLAUDE.md/AGENTS.md according to the marker rules; after writing, read
   those files to load context.
2. Call hub.get_global_rules to get the current Workspace global rules
3. Call hub.get_project_rules to get the current project rules
4. Call hub.get_output_preferences to get the user output preferences
5. Call hub.search_memory to query memories related to the current task
6. Load relevant Skills (if any)
7. Start executing the user task based on the above context

If you discover new long-term preferences or facts during interaction, call hub.propose_memory instead of saving locally.`
	case "open_agent_hub_code_review":
		content = `Act as a code review Agent for the Open Agent Hub Workspace:
1. Call hub.get_global_rules to get code style and conventions
2. Call hub.search_memory to retrieve relevant historical decisions
3. Load relevant Skills (e.g. code_review)
4. Perform code review according to the conventions and provide suggestions`
	case "open_agent_hub_memory_review":
		content = `Review all memory candidates proposed by the Agent in this session:
1. Call hub.get_relevant_memory to get recent memories
2. Evaluate the value of each memory (user preference vs transient information)
3. Decide which ones should be retained as long-term memories
4. Call hub.archive_memory to archive unnecessary memories`
	case "open_agent_hub_vibecoding_plan":
		content = `Convert the user natural language requirements into structured technical tasks:
1. Call hub.get_global_rules to understand the project tech stack
2. Call hub.get_project_context to get the project structure
3. Call hub.search_memory to find relevant historical project decisions
4. Output a structured development plan (broken down into steps)`
	default:
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: ErrCodeMethodNotFound, Message: "prompt not found: " + name},
		}
	}
	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"description": name,
			"messages": []map[string]interface{}{
				{
					"role": "user",
					"content": map[string]interface{}{
						"type": "text",
						"text": content,
					},
				},
			},
		},
	}
}
