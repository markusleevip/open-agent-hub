package database

import (
	"testing"

	"github.com/openagenthub/backend/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newMemDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	if err := db.AutoMigrate(&models.Memory{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

func TestRunMigrations_AppliesAndIsIdempotent(t *testing.T) {
	db := newMemDB(t)

	// First run: apply all migrations and record them
	if err := RunMigrations(db); err != nil {
		t.Fatalf("first run: %v", err)
	}
	var n int64
	db.Model(&SchemaMigration{}).Count(&n)
	if int(n) != len(migrations) {
		t.Fatalf("expected %d migration records, got %d", len(migrations), n)
	}

	// Second run: idempotent, should not duplicate records
	if err := RunMigrations(db); err != nil {
		t.Fatalf("second run: %v", err)
	}
	db.Model(&SchemaMigration{}).Count(&n)
	if int(n) != len(migrations) {
		t.Fatalf("migration record count should not change after re-run, got %d", n)
	}
}

func TestMigration_BackfillCharCount(t *testing.T) {
	db := newMemDB(t)

	// Insert a legacy record with char_count=0 but non-empty content
	m := models.Memory{
		WorkspaceID: "ws1", UserID: "u1", Content: "数据库连接池配置",
		Type: "semantic", Category: "declarative", Scope: "workspace",
		State: "active", CharCount: 0,
	}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run: %v", err)
	}

	var got models.Memory
	db.First(&got, "id = ?", m.ID)
	want := len([]rune(m.Content)) // 8 Chinese characters
	if got.CharCount != want {
		t.Fatalf("char_count should be backfilled to %d, got %d", want, got.CharCount)
	}
}

func TestMigration_BackfillProjectRepoName(t *testing.T) {
	db := newMemDB(t)
	if err := db.AutoMigrate(&models.Project{}); err != nil {
		t.Fatalf("automigrate projects: %v", err)
	}

	// Legacy project with repo_path set but repo_name empty
	p := models.Project{
		OrgID: "o1", WorkspaceID: "ws1", Name: "Super Mario", Slug: "super-mario",
		Status: "active", RepoPath: "/Users/alice/work/super-mario",
	}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run: %v", err)
	}

	var got models.Project
	db.First(&got, "id = ?", p.ID)
	if got.RepoName != "super-mario" {
		t.Fatalf("repo_name should be backfilled to %q, got %q", "super-mario", got.RepoName)
	}
}
