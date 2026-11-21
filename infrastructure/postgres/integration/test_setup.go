package integration

import (
	"context"
	"crypto/rand"
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
	defaultTimeout = 60 * time.Second // setup and cleanup; slow machines (Docker on Windows) need more than 10 s
)

// testServer is the PostgreSQL server the integration tests create their own database on. It defaults to
// localhost:5432 (user and password "trezutil"); TREZUTIL_TEST_PG_HOST, _PORT, _USER and _PASSWORD point the tests at
// a disposable server instead, so they never touch another database server running on the default port.
func testServer(dbname string) string {
	get := func(key, fallback string) string {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
		return fallback
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		get("TREZUTIL_TEST_PG_HOST", "localhost"), get("TREZUTIL_TEST_PG_PORT", "5432"),
		get("TREZUTIL_TEST_PG_USER", "trezutil"), get("TREZUTIL_TEST_PG_PASSWORD", "trezutil"), dbname)
}

// TestDB holds the shared test database connection and helpers.
type TestDB struct {
	DB *sql.DB
}

// NewTestDB creates a test database, applies migrations, and returns a TestDB.
func NewTestDB(tb *testing.T) *TestDB {
	tb.Helper()

	dsn := testServer("postgres")
	adminDB, err := sql.Open("postgres", dsn)
	if err != nil {
		tb.Fatalf("failed to connect to postgres admin DB: %v", err)
	}
	defer adminDB.Close()
	adminDB.SetMaxOpenConns(1)
	adminDB.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid()", testDBName)); err != nil {
		tb.Fatalf("failed to terminate existing connections: %v", err)
	}
	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName)); err != nil {
		tb.Fatalf("failed to drop test database: %v", err)
	}

	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", testDBName)); err != nil {
		tb.Fatalf("failed to create test database: %v", err)
	}

	testDSN := testServer(testDBName)
	db, err := sql.Open("postgres", testDSN)
	if err != nil {
		tb.Fatalf("failed to connect to test database: %v", err)
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(30 * time.Second)

	// Thirty-odd migrations can take longer than the setup timeout on a slow machine (Docker on Windows).
	migCtx, migCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer migCancel()
	if err := applyMigrations(migCtx, db); err != nil {
		tb.Fatalf("failed to apply migrations: %v", err)
	}

	return &TestDB{DB: db}
}

// Close closes the test database connection and drops the database.
func (t *TestDB) Close() {
	if t.DB != nil {
		t.DB.Close()
	}

	dsn := testServer("postgres")
	adminDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return
	}
	defer adminDB.Close()
	adminDB.SetMaxOpenConns(1)
	adminDB.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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
		"payment_ledger",
		"payment_webhook_events",
		"refunds",
		"payments",
		"payment_methods",
		"orders",
		"provider_executions",
		"providers",
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
		"conversation_confirmations",
		"conversation_result_items",
		"conversation_contexts",
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

// uuid generates a random UUID v4 string for use in tests.
func uuid() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// TestDB helper to get a test UUID for test IDs.
func testUUID() string {
	return uuid()
}
