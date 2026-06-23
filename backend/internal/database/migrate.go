package database

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/openagenthub/backend/internal/models"
	"gorm.io/gorm"
)

// Versioned migrations: autoMigrate handles "base table schemas" (create table/add column);
// this file handles "incremental and data migrations" (backfill, conversion, data repair).
// Each migration has a unique ordered ID, recorded in schema_migrations,
// idempotent and safe to re-run.
//
// Convention: IDs follow YYYYMMDD_NN_description, applied in lexicographic order;
// do not change historical IDs or logic once released; always append new migrations
// to the end of the migrations slice.

// SchemaMigration records one applied migration.
type SchemaMigration struct {
	ID        string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	AppliedAt time.Time `json:"applied_at"`
}

func (SchemaMigration) TableName() string { return "schema_migrations" }

// Migration represents one ordered migration. Migrate runs in its own transaction.
type Migration struct {
	ID      string
	Migrate func(tx *gorm.DB) error
}

// migrations is the versioned migration registry (append-only).
var migrations = []Migration{
	{
		// Historical data may contain rows where char_count=0 but content is non-empty;
		// backfill with the character count.
		ID: "20260610_01_backfill_memory_char_count",
		Migrate: func(tx *gorm.DB) error {
			// length() in both SQLite and Postgres counts characters (not bytes).
			return tx.Exec(
				"UPDATE memories SET char_count = length(content) WHERE char_count = 0 AND content <> ''",
			).Error
		},
	},
	{
		// repo_name is used for cross-machine fallback matching; backfill existing projects
		// with the leaf directory name of repo_path.
		// git_remote is empty when the server has no repo info; it will be filled on the next sync_project.
		ID: "20260615_01_backfill_project_repo_name",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("projects") {
				return nil
			}
			var projects []struct {
				ID       string
				RepoPath string
			}
			if err := tx.Table("projects").
				Select("id", "repo_path").
				Where("repo_path <> '' AND (repo_name IS NULL OR repo_name = '')").
				Scan(&projects).Error; err != nil {
				return err
			}
			for _, p := range projects {
				// Avoid importing the services package to prevent database<->services import cycles;
				// compute the leaf directory name in-place.
				clean := strings.TrimRight(filepath.Clean(strings.TrimSpace(p.RepoPath)), "/")
				if clean == "" || clean == "/" {
					continue
				}
				name := filepath.Base(clean)
				if err := tx.Table("projects").Where("id = ?", p.ID).
					Update("repo_name", name).Error; err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		// Platform public Skill initial templates, upserted by slug (idempotent).
		// Public templates are the platform's source of truth, not written into workspace
		// private memories, and do not affect existing user installations.
		ID: "20260617_01_seed_public_skill_templates",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&models.PublicSkillTemplate{}); err != nil {
				return err
			}
			return seedPublicSkillTemplates(tx)
		},
	},
	{
		// Rename the email column in the users table to username,
		// completing the account system migration from email to username.
		ID: "20260618_01_rename_email_to_username",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("users") {
				return nil
			}
			// Skip if the username column already exists (fresh DB or already migrated)
			if tx.Migrator().HasColumn("users", "username") {
				return nil
			}
			// Rename the old email column if it exists
			if tx.Migrator().HasColumn("users", "email") {
				if err := tx.Exec("ALTER TABLE users RENAME COLUMN email TO username").Error; err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		// Backfill the status field of workspace_members to ensure existing data is active.
		ID: "20260618_02_backfill_workspace_member_status",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("workspace_members") {
				return nil
			}
			return tx.Exec("UPDATE workspace_members SET status = 'active' WHERE status = '' OR status IS NULL").Error
		},
	},
	{
		// Deprecated: under the multi-workspace architecture, users can have both
		// personal workspaces (owner) and team workspaces (member) simultaneously,
		// so a single team is no longer enforced. No-op for existing DBs (migration already recorded);
		// no-op for fresh DBs.
		ID: "20260619_01_enforce_single_team",
		Migrate: func(tx *gorm.DB) error {
			return nil
		},
	},
	{
		// After decoupling Agent Profile from workspace, uniqueness is by (user_id, client_type).
		// Clean up historically duplicate agent clients for the same user across different workspaces,
		// keeping only the most recently active record.
		ID: "20260619_02_dedupe_agent_clients",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("agent_clients") {
				return nil
			}
			return tx.Exec(`DELETE FROM agent_clients
WHERE id NOT IN (
  SELECT id FROM (
    SELECT id, ROW_NUMBER() OVER (
      PARTITION BY user_id, client_type
      ORDER BY COALESCE(last_seen_at, first_seen_at) DESC, created_at DESC
    ) AS rn
    FROM agent_clients
  ) ranked
  WHERE rn = 1
)`).Error
		},
	},
	{
		// Workspace gets a new type field (personal | team); backfill existing workspaces as team.
		ID: "20260619_03_backfill_workspace_type",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("workspaces") {
				return nil
			}
			return tx.Exec("UPDATE workspaces SET type = 'team' WHERE type = '' OR type IS NULL").Error
		},
	},
	{
		// OutputPreference is decoupled from (workspace_id, user_id) to just user_id.
		// For each (user_id, key), keep the row with the newest updated_at and delete duplicates.
		ID: "20260619_04_dedup_output_preferences_by_user",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("output_preferences") {
				return nil
			}
			return tx.Exec(`DELETE FROM output_preferences
WHERE id NOT IN (
  SELECT id FROM (
    SELECT id, ROW_NUMBER() OVER (
      PARTITION BY user_id, key
      ORDER BY updated_at DESC, created_at DESC
    ) AS rn
    FROM output_preferences
  ) ranked
  WHERE rn = 1
)`).Error
		},
	},
}

func seedPublicSkillTemplates(tx *gorm.DB) error {
	templates := []models.PublicSkillTemplate{
		{
			Slug:        "go-service-review",
			Name:        "Go Service Review",
			Description: "Review Go service changes for API compatibility, tenant isolation, database behavior, logging, and tests.",
			Content: `# Go Service Review

Use this skill when reviewing or changing Go backend service code.

## Checklist

- Read the real handlers, services, models, middleware, and config before changing behavior.
- Verify API response compatibility, especially the existing code/message/data wrapper.
- Check tenant boundaries on every database query that reads or mutates workspace data.
- Confirm database fields, AutoMigrate behavior, and versioned migrations when models change.
- Prefer focused unit or integration tests for changed business logic.
- Run gofmt and go test ./... before delivery.
`,
			Category:   "backend",
			Tags:       `["go","backend","review"]`,
			Version:    1,
			RiskLevel:  "low",
			Visibility: "public",
			Source:     "platform",
			Status:     "active",
		},
		{
			Slug:        "react-frontend-review",
			Name:        "React Frontend Review",
			Description: "Review React/Vite console changes for API compatibility, loading states, responsive layout, and build health.",
			Content: `# React Frontend Review

Use this skill when changing the React console.

## Checklist

- Match the existing Ant Design page structure and navigation style.
- Keep data fetching scoped to the page action that needs it.
- Avoid request waterfalls when independent data can load together.
- Preserve the API client's code/message/data unwrapping behavior.
- Check empty states, loading states, destructive actions, and disabled states.
- Run npm run build before delivery.
`,
			Category:   "frontend",
			Tags:       `["react","vite","frontend","review"]`,
			Version:    1,
			RiskLevel:  "low",
			Visibility: "public",
			Source:     "platform",
			Status:     "active",
		},
		{
			Slug:        "local-debug-evidence",
			Name:        "Local Debug Evidence",
			Description: "Debug with concrete checkout, config, logs, database rows, and command output before reaching a conclusion.",
			Content: `# Local Debug Evidence

Use this skill when diagnosing local, test, or production-like behavior.

## Workflow

- Identify the exact request, job, user, workspace, or project being investigated.
- Read the real checkout and current config before relying on assumptions.
- Collect command output, logs, database rows, or API responses that prove each step.
- Connect evidence across the same request or entity instead of mixing unrelated samples.
- State what is verified, what remains unverified, and what risk remains.
`,
			Category:   "debugging",
			Tags:       `["debugging","evidence","logs"]`,
			Version:    1,
			RiskLevel:  "medium",
			Visibility: "public",
			Source:     "platform",
			Status:     "active",
		},
		{
			Slug:        "deployment-checklist",
			Name:        "Deployment Checklist",
			Description: "Check configuration, migrations, health endpoints, logs, and rollback posture around a deployment.",
			Content: `# Deployment Checklist

Use this skill when preparing or verifying a deployment.

## Checklist

- Confirm the target environment, branch, commit, and config values.
- Check schema migrations and backwards compatibility before rollout.
- Verify health endpoints and startup logs after deploy.
- Run a narrow smoke test for the changed API or user flow.
- Confirm observability: logs, audit records, and relevant counters.
- Document rollback or mitigation steps for the changed surface.
`,
			Category:   "deployment",
			Tags:       `["deployment","checklist","ops"]`,
			Version:    1,
			RiskLevel:  "medium",
			Visibility: "public",
			Source:     "platform",
			Status:     "active",
		},
	}

	for _, tpl := range templates {
		var existing models.PublicSkillTemplate
		err := tx.Where("slug = ?", tpl.Slug).First(&existing).Error
		if err == nil {
			updates := map[string]interface{}{
				"name":        tpl.Name,
				"description": tpl.Description,
				"content":     tpl.Content,
				"category":    tpl.Category,
				"tags":        tpl.Tags,
				"version":     tpl.Version,
				"risk_level":  tpl.RiskLevel,
				"visibility":  tpl.Visibility,
				"source":      tpl.Source,
				"status":      tpl.Status,
			}
			if err := tx.Model(&existing).Updates(updates).Error; err != nil {
				return err
			}
			continue
		}
		if err := tx.Create(&tpl).Error; err != nil {
			return err
		}
	}
	return nil
}

// RunMigrations applies all unapplied migrations in registry order. Each migration and its record
// are written in the same transaction; any failure rolls back the entire migration
// and does not write to schema_migrations. Safe to call repeatedly (idempotent).
func RunMigrations(db *gorm.DB) error {
	if err := db.AutoMigrate(&SchemaMigration{}); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	var done []SchemaMigration
	if err := db.Find(&done).Error; err != nil {
		return fmt.Errorf("load applied migrations: %w", err)
	}
	applied := make(map[string]bool, len(done))
	for _, d := range done {
		applied[d.ID] = true
	}

	for _, m := range migrations {
		if applied[m.ID] {
			continue
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := m.Migrate(tx); err != nil {
				return err
			}
			return tx.Create(&SchemaMigration{ID: m.ID, AppliedAt: time.Now()}).Error
		}); err != nil {
			return fmt.Errorf("migration %s failed: %w", m.ID, err)
		}
		log.Printf("[migrate] applied %s", m.ID)
	}
	return nil
}
