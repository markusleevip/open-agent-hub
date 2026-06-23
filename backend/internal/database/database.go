package database

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/openagenthub/backend/internal/config"
	"github.com/openagenthub/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Init initializes the database
func Init(cfg *config.Config) error {
	SetEncryptionKey(cfg.EncryptionKey)

	// Ensure the data directory exists
	if cfg.DBType == "sqlite" {
		dir := filepath.Dir(cfg.DBDSN)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	gormConfig := &gorm.Config{
		Logger: logger.New(
			log.New(os.Stdout, "[GORM] ", log.LstdFlags),
			logger.Config{
				SlowThreshold:             200 * time.Millisecond,
				LogLevel:                  logger.Warn,
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		),
	}

	var err error
	if cfg.DBType == "sqlite" {
		DB, err = gorm.Open(sqlite.Open(cfg.DBDSN), gormConfig)
	} else if cfg.DBType == "postgres" {
		DB, err = gorm.Open(postgres.Open(cfg.DBDSN), gormConfig)
	} else {
		log.Fatalf("unsupported DB type: %s (use 'sqlite' or 'postgres')", cfg.DBType)
	}

	if err != nil {
		return err
	}

	// Enable SQLite foreign keys
	if cfg.DBType == "sqlite" {
		DB.Exec("PRAGMA foreign_keys = ON;")
	}

	// Pre-migration: rename the email column to username before AutoMigrate
	// (SQLite does not allow adding NOT NULL columns to tables with existing data, so rename first)
	if err := preMigrateRenameEmailToUsername(); err != nil {
		return err
	}

	// Auto-migrate (base table schemas)
	if err := autoMigrate(); err != nil {
		return err
	}

	// Versioned migrations (incremental/data migrations, recorded in schema_migrations)
	if err := RunMigrations(DB); err != nil {
		return err
	}

	// Bootstrap initial data
	if err := bootstrap(cfg); err != nil {
		return err
	}

	return nil
}

// preMigrateRenameEmailToUsername runs before AutoMigrate,
// renaming the email column in the users table to username.
// Must run before AutoMigrate because SQLite does not allow adding NOT NULL columns to tables with existing data.
func preMigrateRenameEmailToUsername() error {
	if !DB.Migrator().HasTable("users") {
		return nil
	}
	// Skip if the username column already exists (fresh DB or already migrated)
	if DB.Migrator().HasColumn("users", "username") {
		return nil
	}
	// Rename the old email column if it exists
	if DB.Migrator().HasColumn("users", "email") {
		if err := DB.Exec("ALTER TABLE users RENAME COLUMN email TO username").Error; err != nil {
			return err
		}
		log.Println("[pre-migrate] renamed users.email → users.username")
	}
	return nil
}

func autoMigrate() error {
	return DB.AutoMigrate(
		// Tenants
		&models.Organization{},
		&models.Workspace{},
		&models.User{},
		&models.WorkspaceMember{},
		&models.Project{},
		&models.SyncRecord{},
		// Rules & configuration
		&models.Rule{},
		&models.OutputPreference{},
		// Memories
		&models.Memory{},
		&models.MemoryValidity{},
		&models.MemoryMapping{},
		&models.MemorySnapshot{},
		&models.MemoryAccessLog{},
		&models.SkillCurationLog{},
		&models.PublicSkillTemplate{},
		&models.SkillInstall{},
		// Agent & Session
		&models.AgentClient{},
		&models.MCPSession{},
		// Tool
		&models.ConnectedMCPServer{},
		&models.ToolPolicy{},
		&models.ToolInvocationLog{},
		// Auth
		&models.APIKey{},
		&models.OAuthToken{},
		&models.UsageRecord{},
		&models.AuditLog{},
	)
}

// bootstrap initializes seed data
func bootstrap(cfg *config.Config) error {
	// Create default organization
	var org models.Organization
	if err := DB.First(&org).Error; err == nil {
		return nil // data already exists
	}

	org = models.Organization{
		Name:   "Default Organization",
		Slug:   "default",
		Plan:   "pro",
		Status: "active",
	}
	if err := DB.Create(&org).Error; err != nil {
		return err
	}

	// Create bootstrap user
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.BootstrapPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := models.User{
		Username:     cfg.BootstrapUsername,
		PasswordHash: string(hash),
		DisplayName:  "Admin",
		Status:       "active",
	}
	if err := DB.Create(&user).Error; err != nil {
		return err
	}

	// Create default workspace
	ws := models.Workspace{
		OrgID:              org.ID,
		Name:               "Default Workspace",
		Slug:               "default",
		QuotaMemoryCount:   10000,
		QuotaToolCallDaily: 5000,
		Status:             "active",
	}
	if err := DB.Create(&ws).Error; err != nil {
		return err
	}

	// Add as Owner
	now := time.Now()
	member := models.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      user.ID,
		Role:        "owner",
		Status:      "active",
		InvitedAt:   now,
		JoinedAt:    &now,
	}
	if err := DB.Create(&member).Error; err != nil {
		return err
	}

	// Create default Global Rules (example data)
	defaultRules := []models.Rule{
		{
			OrgID:       org.ID,
			WorkspaceID: ws.ID,
			Name:        "Backend tech stack",
			Description: "Backend services use Go + the Gin framework",
			Value:       "Backend uses Go 1.21+ and the Gin framework; all APIs must return the unified response format",
			Type:        "style_guide",
			Tags:        `["tech_stack","backend"]`,
			Scope:       "workspace",
			Version:     1,
		},
		{
			OrgID:       org.ID,
			WorkspaceID: ws.ID,
			Name:        "Frontend tech stack",
			Description: "Frontend uses React + TypeScript + Ant Design",
			Value:       "Frontend uses React 19 + TypeScript + Vite + Ant Design 5.x",
			Type:        "style_guide",
			Tags:        `["tech_stack","frontend"]`,
			Scope:       "workspace",
			Version:     1,
		},
		{
			OrgID:       org.ID,
			WorkspaceID: ws.ID,
			Name:        "API error handling convention",
			Description: "All API endpoints must handle errors and return the standard format",
			Value:       "All endpoints must handle errors and return the { code, message, data } format",
			Type:        "output_format",
			Tags:        `["api","error_handling"]`,
			Scope:       "workspace",
			Version:     1,
		},
	}
	for _, r := range defaultRules {
		DB.Create(&r)
	}

	// Create example memories
	exampleMemories := []models.Memory{
		{
			OrgID:       org.ID,
			WorkspaceID: ws.ID,
			UserID:      user.ID,
			Content:     "User prefers building the SaaS platform with Go + React",
			Type:        "user_preference",
			Category:    "declarative",
			Tags:        `["tech_stack","preference"]`,
			Scope:       "workspace",
			Provenance:  "human_curated",
			Importance:  0.9,
			Pinned:      true,
			State:       "active",
			CharCount:   55,
		},
		{
			OrgID:       org.ID,
			WorkspaceID: ws.ID,
			UserID:      user.ID,
			Content:     "Backend APIs must return the unified code/message/data format",
			Type:        "semantic",
			Category:    "declarative",
			Tags:        `["api","convention"]`,
			Scope:       "workspace",
			Provenance:  "human_curated",
			Importance:  0.85,
			Pinned:      true,
			State:       "active",
			CharCount:   61,
		},
		{
			OrgID:       org.ID,
			WorkspaceID: ws.ID,
			UserID:      user.ID,
			Content:     "VibeCoding project bootstrap: 1) read rules 2) load project context 3) retrieve relevant memories 4) start working",
			Type:        "skill",
			Category:    "procedural",
			Tags:        `["vibecoding","init"]`,
			Scope:       "workspace",
			Provenance:  "human_curated",
			Importance:  0.8,
			State:       "active",
			CharCount:   114,
		},
	}
	for _, m := range exampleMemories {
		DB.Create(&m)
	}

	// Create an example Connected MCP Server
	server := models.ConnectedMCPServer{
		WorkspaceID: ws.ID,
		Name:        "github",
		DisplayName: "GitHub MCP (example)",
		Endpoint:    "https://mcp.github.example.com",
		Transport:   "streamable_http",
		AuthType:    "api_key",
		ToolsJSON:   `[{"name":"github.create_pull_request","description":"Create a PR"},{"name":"github.list_repos","description":"List repos"}]`,
		PolicyJSON:  `{}`,
		Status:      "inactive",
	}
	DB.Create(&server)

	// Default Tool Policy
	policies := []models.ToolPolicy{
		{
			WorkspaceID:          ws.ID,
			ConnectedServerID:    server.ID,
			ToolName:             "github.create_pull_request",
			Allowed:              true,
			RequiresConfirmation: true,
			MaxCallsPerDay:       50,
			RiskLevel:            "high",
		},
		{
			WorkspaceID:          ws.ID,
			ConnectedServerID:    server.ID,
			ToolName:             "github.list_repos",
			Allowed:              true,
			RequiresConfirmation: false,
			MaxCallsPerDay:       0,
			RiskLevel:            "low",
		},
	}
	for _, p := range policies {
		DB.Create(&p)
	}

	// Create example MCP Token (pat_)
	createExampleToken(ws.ID, user.ID, "Default Token", []string{"read", "write", "admin"})

	// Create example Agent Client
	ac := models.AgentClient{
		WorkspaceID:   ws.ID,
		UserID:        user.ID,
		ClientType:    "cursor",
		ClientName:    "Demo Cursor",
		ClientVersion: "0.42.0",
		Status:        "active",
		FirstSeenAt:   now,
		LastSeenAt:    &now,
	}
	DB.Create(&ac)

	log.Println("==============================================")
	log.Println("Bootstrap data created:")
	log.Println("  Username: ", cfg.BootstrapUsername)
	log.Println("  Password: ", cfg.BootstrapPassword)
	log.Println("  Workspace: Default Workspace")
	log.Println("==============================================")
	return nil
}

func createExampleToken(workspaceID, userID, name string, scopes []string) {
	// Skip if already exists
	var existing models.APIKey
	if err := DB.Where("workspace_id = ? AND name = ?", workspaceID, name).First(&existing).Error; err == nil {
		return
	}

	token := "pat_" + randomString(32)
	prefix := token[:11] // "pat_xxxxxx"
	hash := bcryptHashForToken(token)

	key := models.APIKey{
		WorkspaceID: workspaceID,
		Name:        name,
		Prefix:      prefix,
		Hash:        hash,
		Scopes:      stringArrayToJSON(scopes),
		CreatedBy:   userID,
	}
	DB.Create(&key)

	log.Printf("  Example MCP Token: %s (prefix: %s)", token, prefix)
}
