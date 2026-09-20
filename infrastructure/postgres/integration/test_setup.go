package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

const (
	testDBName     = "trezutil_test"
	migrationsDir  = "../../../migrations"
	defaultTimeout = 10 * time.Second
)

// TestDB holds the shared test database connection and helpers.
type TestDB struct {
	DB *sql.DB
}

// NewTestDB creates a test database, applies migrations, and returns a TestDB.
func NewTestDB(tb *testing.T) *TestDB {
	tb.Helper()

	dsn := fmt.Sprintf("host=localhost port=5432 user=openpg password=openpgpwd dbname=postgres sslmode=disable")
	adminDB, err := sql.Open("postgres", dsn)
	if err != nil {
		tb.Fatalf("failed to connect to postgres admin DB: %v", err)
	}
	defer adminDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	adminDB.ExecContext(ctx, fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid()", testDBName))
	adminDB.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))

	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", testDBName)); err != nil {
		tb.Fatalf("failed to create test database: %v", err)
	}

	testDSN := fmt.Sprintf("host=localhost port=5432 user=openpg password=openpgpwd dbname=%s sslmode=disable", testDBName)
	db, err := sql.Open("postgres", testDSN)
	if err != nil {
		tb.Fatalf("failed to connect to test database: %v", err)
	}

	if err := applyMigrations(ctx, db); err != nil {
		tb.Fatalf("failed to apply migrations: %v", err)
	}

	return &TestDB{DB: db}
}

// Close closes the test database connection and drops the database.
func (t *TestDB) Close() {
	if t.DB != nil {
		t.DB.Close()
	}

	dsn := fmt.Sprintf("host=localhost port=5432 user=openpg password=openpgpwd dbname=postgres sslmode=disable")
	adminDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return
	}
	defer adminDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	adminDB.ExecContext(ctx, fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid()", testDBName))
	adminDB.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
}

// applyMigrations runs all SQL migration files in sorted order.
func applyMigrations(ctx context.Context, db *sql.DB) error {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		return err
	}

	sort.Strings(files)

	for _, file := range files {
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", file, err)
		}

		upSQL := extractUpSection(string(sqlBytes))
		if _, err := db.ExecContext(ctx, upSQL); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", file, err)
		}
	}

	return nil
}

// extractUpSection extracts the SQL between -- +goose Up and -- +goose Down.
func extractUpSection(content string) string {
	upMarker := "-- +goose Up"
	downMarker := "-- +goose Down"

	upIdx := strings.Index(content, upMarker)
	downIdx := strings.Index(content, downMarker)

	if upIdx == -1 {
		return content
	}

	start := upIdx + len(upMarker)
	if downIdx == -1 {
		return strings.TrimSpace(content[start:])
	}

	return strings.TrimSpace(content[start:downIdx])
}

// Cleanup truncates all application tables between tests.
func (t *TestDB) Cleanup(tb *testing.T) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	tables := []string{
		"role_permissions",
		"user_roles",
		"permissions",
		"roles",
		"external_identities",
		"user_locations",
		"listing_locations",
		"listing_availability",
		"listing_media",
		"listing_attributes",
		"listings",
		"attribute_definitions",
		"categories",
		"locations",
		"stores",
		"seller_profiles",
		"organization_members",
		"organizations",
		"users",
		"conversations",
		"webhook_events",
		"channel_accounts",
		"messages",
	}

	for _, table := range tables {
		if _, err := t.DB.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)); err != nil {
			tb.Fatalf("failed to truncate %s: %v", table, err)
		}
	}
}
