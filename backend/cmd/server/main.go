package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/openagenthub/backend/internal/config"
	"github.com/openagenthub/backend/internal/database"
	"github.com/openagenthub/backend/internal/handlers"
	"github.com/openagenthub/backend/internal/mcp"
	"github.com/openagenthub/backend/internal/middleware"
)

func main() {
	cfg := config.Load()

	// Startup security self-check: warn about config items still using dev defaults
	for _, warning := range cfg.SecurityWarnings() {
		log.Printf("⚠️  SECURITY WARNING: %s", warning)
	}

	// Initialize database
	if err := database.Init(cfg); err != nil {
		log.Fatalf("database initialization failed: %v", err)
	}

	// Register GORM multi-tenant plugin: auto-inject WHERE workspace_id = ? for tables with that column
	if err := database.DB.Use(middleware.NewTenantPlugin(middleware.TenantContextKey)); err != nil {
		log.Fatalf("tenant plugin registration failed: %v", err)
	}

	// Start Console (REST API)
	consoleRouter := buildConsoleRouter(cfg)
	consoleServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.ConsolePort),
		Handler: consoleRouter,
	}

	// Start MCP Gateway
	mcpRouter := buildMCPRouter(cfg)
	mcpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.MCPPort),
		Handler: mcpRouter,
	}

	// Start both servers
	go func() {
		log.Printf("🚀 SaaS Console (REST API) listening on :%d", cfg.ConsolePort)
		if err := consoleServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Console server error: %v", err)
		}
	}()
	go func() {
		log.Printf("🔌 MCP Gateway listening on :%d", cfg.MCPPort)
		if err := mcpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("MCP server error: %v", err)
		}
	}()

	// Health check banner
	log.Println("===================================================")
	log.Println("  Open Agent Hub - Backend")
	log.Println("===================================================")
	log.Printf("  Console: http://localhost:%d", cfg.ConsolePort)
	log.Printf("  MCP:     http://localhost:%d/mcp", cfg.MCPPort)
	log.Printf("  Default username: %s", cfg.BootstrapUsername)
	log.Printf("  Default password: %s", cfg.BootstrapPassword)
	log.Println("===================================================")

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("received termination signal, shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := consoleServer.Shutdown(ctx); err != nil {
		log.Printf("Console shutdown error: %v", err)
	}
	if err := mcpServer.Shutdown(ctx); err != nil {
		log.Printf("MCP shutdown error: %v", err)
	}
	log.Println("server stopped")
}

// buildConsoleRouter SaaS Console REST API
func buildConsoleRouter(cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Mcp-Session-Id", "X-Project-Path"},
		ExposeHeaders:    []string{"ETag", "Cache-Control", "Last-Modified"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "open-agent-hub-console"})
	})

	// Unauthenticated API
	api := r.Group("/api")
	authH := handlers.NewAuthHandler(cfg)
	api.POST("/auth/login", authH.Login)
	api.POST("/auth/register", authH.Register)

	// Authenticated API
	authed := api.Group("")
	authed.Use(middleware.AuthRequired(cfg))
	{
		authed.GET("/auth/me", authH.Me)
		authed.POST("/auth/switch-workspace", authH.SwitchWorkspace)

		// Workspace
		wsH := handlers.NewWorkspaceHandler(cfg)
		authed.GET("/workspaces", wsH.List)
		authed.GET("/workspaces/:id", wsH.Get)
		authed.POST("/workspaces", wsH.Create)
		authed.PUT("/workspaces/:id", wsH.Update)
		authed.DELETE("/workspaces/:id", wsH.Delete)

		// Members
		memH := handlers.NewMemberHandler()
		authed.GET("/members", memH.ListMembers)
		authed.POST("/members", memH.InviteMember)
		authed.PUT("/members/:id", memH.UpdateMemberRole)
		authed.DELETE("/members/:id", memH.RemoveMember)

		// My Invitations (cross-workspace)
		authed.GET("/my-invitations", memH.ListMyInvitations)
		authed.POST("/my-invitations/:id/accept", memH.AcceptInvitation)
		authed.POST("/my-invitations/:id/reject", memH.RejectInvitation)

		// Leave workspace (non-owner)
		authed.POST("/workspaces/:id/leave", memH.LeaveWorkspace)

		// Rules
		ruleH := handlers.NewRuleHandler()
		authed.GET("/rules", ruleH.List)
		authed.GET("/rules/:id", ruleH.Get)
		authed.POST("/rules", ruleH.Create)
		authed.PUT("/rules/:id", ruleH.Update)
		authed.DELETE("/rules/:id", ruleH.Delete)
		authed.GET("/rules/global", ruleH.GetGlobalRules)
		authed.GET("/rules/project", ruleH.GetProjectRules)
		authed.GET("/workspace-policy", ruleH.GetWorkspacePolicy)
		authed.GET("/output-preferences", ruleH.GetOutputPreferences)
		authed.GET("/personal-instructions", ruleH.GetPersonalInstructions)
		authed.PUT("/personal-instructions", ruleH.UpdatePersonalInstructions)

		// Memories
		memH2 := handlers.NewMemoryHandler()
		authed.GET("/memories", memH2.List)
		authed.GET("/memories/:id", memH2.Get)
		authed.POST("/memories", memH2.Create)
		authed.PUT("/memories/:id", memH2.Update)
		authed.DELETE("/memories/:id", memH2.Delete)
		authed.POST("/memories/:id/archive", memH2.Archive)
		authed.POST("/memories/:id/review", memH2.Review)
		authed.POST("/memories/search", memH2.Search)
		authed.GET("/memories/stats", memH2.Stats)

		// Skills
		skillH := handlers.NewSkillHandler()
		authed.GET("/skills", skillH.List)
		authed.POST("/skills", skillH.Create)
		authed.PUT("/skills/:id/state", skillH.ChangeState)

		publicSkillH := handlers.NewPublicSkillHandler()
		authed.GET("/public-skills", publicSkillH.List)
		authed.GET("/public-skills/:id", publicSkillH.Get)
		publicSkillAdmin := authed.Group("/public-skills")
		publicSkillAdmin.Use(middleware.RequireRole("owner", "admin"))
		{
			publicSkillAdmin.POST("", publicSkillH.Create)
			publicSkillAdmin.PUT("/:id", publicSkillH.Update)
			publicSkillAdmin.PUT("/:id/status", publicSkillH.ChangeStatus)
		}

		skillInstallH := handlers.NewSkillInstallHandler()
		authed.GET("/skill-installs", skillInstallH.List)
		authed.POST("/skill-installs", skillInstallH.Create)
		authed.PUT("/skill-installs/:id/state", skillInstallH.ChangeState)
		authed.POST("/skill-installs/:id/upgrade", skillInstallH.Upgrade)

		// Projects
		projH := handlers.NewProjectHandler()
		authed.GET("/projects", projH.List)
		authed.GET("/projects/:id", projH.Get)
		authed.GET("/projects/:id/sync-records", projH.SyncRecords)
		authed.POST("/projects", projH.Create)
		authed.PUT("/projects/:id", projH.Update)
		authed.DELETE("/projects/:id", projH.Delete)

		// Tokens
		tokH := handlers.NewTokenHandler(cfg)
		authed.GET("/tokens", tokH.List)
		authed.POST("/tokens", tokH.Create)
		authed.POST("/tokens/:id/revoke", tokH.Revoke)
		authed.DELETE("/tokens/:id", tokH.Delete)

		// Agent Clients
		acH := handlers.NewAgentClientHandler()
		authed.GET("/agent-clients", acH.List)
		authed.GET("/agent-clients/:id", acH.Get)
		authed.DELETE("/agent-clients/:id", acH.Delete)

		// Connected MCP Servers
		srvH := handlers.NewConnectedServerHandler()
		authed.GET("/connected-servers", srvH.List)
		authed.GET("/connected-servers/:id", srvH.Get)
		authed.POST("/connected-servers", srvH.Create)
		authed.PUT("/connected-servers/:id", srvH.Update)
		authed.DELETE("/connected-servers/:id", srvH.Delete)

		// Tool Policies
		tpH := handlers.NewToolPolicyHandler()
		authed.GET("/tool-policies", tpH.List)
		authed.PUT("/tool-policies/:id", tpH.Update)

		// Tool Invocation Logs
		tiH := handlers.NewToolInvocationLogHandler()
		authed.GET("/tool-invocation-logs", tiH.List)

		// Usage
		uH := handlers.NewUsageHandler()
		authed.GET("/usage/dashboard", uH.Dashboard)

		// Audit
		aH := handlers.NewAuditHandler()
		authed.GET("/audit-logs", aH.List)
	}

	return r
}

// buildMCPRouter MCP Gateway
func buildMCPRouter(cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "Mcp-Session-Id", "X-Project-Path"},
		ExposeHeaders:    []string{"ETag", "Cache-Control", "Last-Modified", "Mcp-Session-Id"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "open-agent-hub-mcp"})
	})

	gw := mcp.NewGateway(cfg)
	r.POST("/mcp", gw.HandleHTTP)
	r.GET("/mcp", gw.HandleSSE)
	// Legacy SSE transport (Cursor, Claude Desktop, etc.)
	r.GET("/sse", gw.HandleLegacySSE)
	r.POST("/message", gw.HandleLegacyMessage)

	return r
}
