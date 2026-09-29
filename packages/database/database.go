package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	// PostgreSQL driver.
	_ "github.com/lib/pq"
)

// DevDatabaseURL is used only when APP_ENV=development and DATABASE_URL is unset.
const DevDatabaseURL = "postgres://trezutil:trezutil@localhost:5432/trezutil?sslmode=disable"

// IsDevelopment reports whether the process runs with APP_ENV=development.
func IsDevelopment() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "development")
}

// URLFromEnv returns DATABASE_URL. Outside development it is mandatory so a
// misconfigured deployment fails fast instead of silently using dev defaults.
func URLFromEnv() (string, error) {
	if v := strings.TrimSpace(os.Getenv("DATABASE_URL")); v != "" {
		return v, nil
	}
	if IsDevelopment() {
		return DevDatabaseURL, nil
	}
	return "", errors.New("DATABASE_URL is required (set APP_ENV=development to use the local default)")
}

// Open connects to PostgreSQL using DATABASE_URL, configures the pool and
// waits (bounded) for the database to accept connections.
func Open(ctx context.Context) (*sql.DB, error) {
	url, err := URLFromEnv()
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	deadline := time.Now().Add(30 * time.Second)
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = db.PingContext(pingCtx)
		cancel()
		if err == nil {
			return db, nil
		}
		if time.Now().After(deadline) {
			_ = db.Close()
			return nil, fmt.Errorf("database not reachable: %w", err)
		}
		time.Sleep(time.Second)
	}
}
