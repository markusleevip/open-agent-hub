package middleware

import (
	"context"

	"github.com/openagenthub/backend/internal/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var tenantSkippedTables = map[string]bool{
	"organizations":        true,
	"users":                true,
	"oauth_tokens":         true,
	"schema_migrations":    true,
	"output_preferences":   true,
	"public_skill_templates": true,
}

type TenantPlugin struct {
	workspaceKey string
}

func NewTenantPlugin(workspaceKey string) *TenantPlugin {
	return &TenantPlugin{workspaceKey: workspaceKey}
}

func (p *TenantPlugin) Name() string { return "tenant-plugin" }

func (p *TenantPlugin) Initialize(db *gorm.DB) error {
	if err := db.Callback().Query().Before("gorm:query").Register("tenant:query", p.beforeQuery); err != nil {
		return err
	}
	if err := db.Callback().Update().Before("gorm:update").Register("tenant:update", p.beforeUpdate); err != nil {
		return err
	}
	if err := db.Callback().Delete().Before("gorm:delete").Register("tenant:delete", p.beforeDelete); err != nil {
		return err
	}
	return nil
}

func (p *TenantPlugin) beforeQuery(db *gorm.DB) {
	p.injectWorkspaceFilter(db)
}

func (p *TenantPlugin) beforeUpdate(db *gorm.DB) {
	p.injectWorkspaceFilter(db)
}

func (p *TenantPlugin) beforeDelete(db *gorm.DB) {
	p.injectWorkspaceFilter(db)
}

func (p *TenantPlugin) injectWorkspaceFilter(db *gorm.DB) {
	if db.Statement.Schema == nil {
		return
	}
	tableName := db.Statement.Schema.Table
	if tenantSkippedTables[tableName] {
		return
	}

	hasWorkspaceID := false
	for _, field := range db.Statement.Schema.Fields {
		if field.DBName == "workspace_id" {
			hasWorkspaceID = true
			break
		}
	}
	if !hasWorkspaceID {
		return
	}

	workspaceID, ok := db.Statement.Context.Value(p.workspaceKey).(string)
	if !ok || workspaceID == "" {
		return
	}

	db.Statement.AddClause(clause.Where{
		Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Name: "workspace_id"}, Value: workspaceID},
		},
	})
}

// TenantContextKey is the context key used by TenantPlugin to read workspace_id.
// Must match the key passed to NewTenantPlugin in main.go.
const TenantContextKey = "workspace_id"

// WorkspaceDB returns a GORM session with workspace_id injected into context,
// so that TenantPlugin automatically adds WHERE workspace_id = ? to queries.
func WorkspaceDB(workspaceID string) *gorm.DB {
	ctx := context.WithValue(context.Background(), TenantContextKey, workspaceID)
	return database.DB.Session(&gorm.Session{Context: ctx})
}
