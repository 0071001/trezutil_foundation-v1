// Command migrate applies the SQL migrations in MIGRATIONS_DIR (default
// ./migrations) to DATABASE_URL, in lexical order, each in its own
// transaction, recording applied versions in schema_migrations.
//
// Usage:
//
//	migrate                 apply all pending migrations
//	migrate -status         list applied/pending migrations
//	migrate -baseline 000017  mark migrations up to 000017 as applied
//	                          (for databases created before this tool existed)
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/trezutil/trezutil/packages/common"
	"github.com/trezutil/trezutil/packages/database"
)

func main() {
	status := flag.Bool("status", false, "print migration status and exit")
	baseline := flag.String("baseline", "", "mark all migrations with version <= this as applied without running them")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	db, err := database.Open(ctx)
	if err != nil {
		log.Fatalf("migrate: %v", err)
	}
	defer db.Close()

	dir := common.Getenv("MIGRATIONS_DIR", "./migrations")
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil || len(files) == 0 {
		log.Fatalf("migrate: no migrations found in %s", dir)
	}
	sort.Strings(files)

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW())`); err != nil {
		log.Fatalf("migrate: create schema_migrations: %v", err)
	}
	applied, err := appliedVersions(ctx, db)
	if err != nil {
		log.Fatalf("migrate: %v", err)
	}

	for _, file := range files {
		version := versionOf(file)
		switch {
		case *status:
			state := "pending"
			if applied[version] {
				state = "applied"
			}
			fmt.Printf("%-8s %s\n", state, filepath.Base(file))
		case applied[version]:
			continue
		case *baseline != "" && version <= *baseline:
			if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT DO NOTHING`, version); err != nil {
				log.Fatalf("migrate: baseline %s: %v", version, err)
			}
			fmt.Printf("baselined %s\n", filepath.Base(file))
		default:
			if err := apply(ctx, db, file, version); err != nil {
				log.Fatalf("migrate: %s: %v", filepath.Base(file), err)
			}
			fmt.Printf("applied   %s\n", filepath.Base(file))
		}
	}
}

func apply(ctx context.Context, db *sql.DB, file, version string) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, upSection(string(content))); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func appliedVersions(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

// versionOf returns the numeric prefix of a migration file name ("000018").
func versionOf(file string) string {
	base := filepath.Base(file)
	if i := strings.Index(base, "_"); i > 0 {
		return base[:i]
	}
	return strings.TrimSuffix(base, ".sql")
}

// upSection extracts the SQL between "-- +goose Up" and "-- +goose Down".
func upSection(content string) string {
	up := strings.Index(content, "-- +goose Up")
	if up == -1 {
		return content
	}
	rest := content[up+len("-- +goose Up"):]
	if down := strings.Index(rest, "-- +goose Down"); down != -1 {
		rest = rest[:down]
	}
	return strings.TrimSpace(rest)
}
