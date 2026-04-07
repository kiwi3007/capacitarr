package services

import (
	"testing"

	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ─── Schema validation tests ────────────────────────────────────────────────
// These tests exercise the SchemaService that runs at startup to verify
// database integrity and schema correctness. They use the same in-memory
// SQLite setup as other service tests (Goose migrations + fixups).

// TestSchemaValidateAndRepair_ValidSchema verifies that a freshly migrated
// database passes the full ValidateAndRepair cycle with no errors.
func TestSchemaValidateAndRepair_ValidSchema(t *testing.T) {
	database := setupTestDB(t)
	svc := NewSchemaService(database)

	report, err := svc.ValidateAndRepair()
	if err != nil {
		t.Fatalf("ValidateAndRepair returned error: %v", err)
	}

	if !report.IntegrityOK {
		t.Error("expected IntegrityOK=true on fresh database")
	}

	if len(report.Errors) != 0 {
		for _, e := range report.Errors {
			t.Errorf("unexpected schema error: %s", e.String())
		}
		t.Fatalf("expected 0 schema errors, got %d", len(report.Errors))
	}
}

// TestSchemaValidate_MissingColumn verifies that the validator detects a
// missing column when one is dropped from an existing table.
func TestSchemaValidate_MissingColumn(t *testing.T) {
	database := setupTestDB(t)
	svc := NewSchemaService(database)

	// Drop the "item_count" column from media_cache by recreating the table
	// without it. SQLite does not support DROP COLUMN on older versions, so
	// we use the standard SQLite table-rebuild pattern.
	stmts := []string{
		"CREATE TABLE media_cache_backup (id INTEGER PRIMARY KEY, preview_json TEXT NOT NULL, updated_at DATETIME)",
		"INSERT INTO media_cache_backup SELECT id, preview_json, updated_at FROM media_cache",
		"DROP TABLE media_cache",
		"ALTER TABLE media_cache_backup RENAME TO media_cache",
	}
	for _, stmt := range stmts {
		if err := database.Exec(stmt).Error; err != nil {
			t.Fatalf("Failed to tamper with media_cache: %v (stmt: %s)", err, stmt)
		}
	}

	// Run validate() directly — we skip repair() since it would fix the issue.
	errors := svc.validate()

	// Expect at least one missing_column error for media_cache.item_count
	found := false
	for _, e := range errors {
		if e.Table == "media_cache" && e.Column == "item_count" && e.Problem == "missing_column" {
			found = true
		}
	}
	if !found {
		t.Error("expected missing_column error for media_cache.item_count")
		for _, e := range errors {
			t.Logf("  got: %s", e.String())
		}
	}
}

// TestSchemaValidate_ExtraColumnTolerated verifies that an extra column in
// the database (not in the GORM model) does not produce an error. GORM
// ignores extra columns, so the validator should too.
func TestSchemaValidate_ExtraColumnTolerated(t *testing.T) {
	database := setupTestDB(t)
	svc := NewSchemaService(database)

	// Add an extra column to media_cache that has no corresponding GORM field.
	if err := database.Exec("ALTER TABLE media_cache ADD COLUMN extra_column TEXT DEFAULT 'Serenity'").Error; err != nil {
		t.Fatalf("Failed to add extra column: %v", err)
	}

	errors := svc.validate()

	// No errors should be reported — extra columns are tolerated.
	for _, e := range errors {
		if e.Table == "media_cache" {
			t.Errorf("unexpected error on media_cache with extra column: %s", e.String())
		}
	}

	if len(errors) != 0 {
		t.Errorf("expected 0 schema errors, got %d", len(errors))
		for _, e := range errors {
			t.Logf("  got: %s", e.String())
		}
	}
}

// TestSchemaValidate_MissingTable verifies that the validator detects a
// completely missing table.
func TestSchemaValidate_MissingTable(t *testing.T) {
	database := setupTestDB(t)
	svc := NewSchemaService(database)

	// Drop the media_cache table entirely.
	if err := database.Exec("DROP TABLE media_cache").Error; err != nil {
		t.Fatalf("Failed to drop media_cache table: %v", err)
	}

	errors := svc.validate()

	// Expect a missing_table error for media_cache
	found := false
	for _, e := range errors {
		if e.Table == "media_cache" && e.Problem == "missing_table" {
			found = true
		}
	}
	if !found {
		t.Error("expected missing_table error for media_cache")
		for _, e := range errors {
			t.Logf("  got: %s", e.String())
		}
	}
}

// TestSchemaError_String verifies the human-readable output of SchemaError.
func TestSchemaError_String(t *testing.T) {
	tests := []struct {
		name     string
		err      SchemaError
		expected string
	}{
		{
			name:     "missing table",
			err:      SchemaError{Table: "media_cache", Problem: "missing_table"},
			expected: "missing_table: media_cache",
		},
		{
			name:     "missing column",
			err:      SchemaError{Table: "media_cache", Column: "item_count", Problem: "missing_column"},
			expected: "missing_column: media_cache.item_count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.String()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// TestSchemaRepair_RecreatesMissingTable verifies that repair() recreates a
// missing table via AutoMigrate. This test uses a pure GORM-created schema
// (AutoMigrate only, no Goose) because Goose-created schemas have minor DDL
// differences that cause AutoMigrate's temp-table recreation to fail on SQLite.
func TestSchemaRepair_RecreatesMissingTable(t *testing.T) {
	// Set up a pure GORM database (AutoMigrate only, no Goose migrations).
	database, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open in-memory SQLite: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("Failed to get underlying sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	// Create all tables via GORM AutoMigrate (clean schema, no Goose).
	if err := database.AutoMigrate(allModels...); err != nil {
		t.Fatalf("Initial AutoMigrate failed: %v", err)
	}

	// Drop media_cache so repair() has something to fix.
	if err := database.Exec("DROP TABLE media_cache").Error; err != nil {
		t.Fatalf("Failed to drop media_cache table: %v", err)
	}

	svc := NewSchemaService(database)
	repairs := svc.repair()

	// repair() should have recreated media_cache
	foundRepair := false
	for _, r := range repairs {
		if r == "created table media_cache" {
			foundRepair = true
		}
	}
	if !foundRepair {
		t.Errorf("expected repair to include 'created table media_cache', got: %v", repairs)
	}

	// After repair, validate() should report no errors for media_cache
	errors := svc.validate()
	for _, e := range errors {
		if e.Table == "media_cache" {
			t.Errorf("unexpected error on media_cache after repair: %s", e.String())
		}
	}
}
