// Package db provides the SQLite connection and PRAGMA configuration.
package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// connPragmas are applied to every new SQLite connection opened by the pool.
// WAL mode persists at the database level; the rest are per-connection settings.
var connPragmas = []string{
	"PRAGMA journal_mode = WAL",
	"PRAGMA foreign_keys = ON",
	"PRAGMA busy_timeout = 5000",
	"PRAGMA synchronous = NORMAL",
}

// pragmaConnector wraps the sqlite driver, applying connPragmas to every new
// connection in the pool. This lets the pool use multiple concurrent connections
// in WAL mode without losing per-connection settings, so background data
// hydration cannot block HTTP request handlers.
type pragmaConnector struct {
	dsn string
	drv driver.Driver
}

func (pc *pragmaConnector) Connect(_ context.Context) (driver.Conn, error) {
	conn, err := pc.drv.Open(pc.dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open connection: %w", err)
	}
	for _, q := range connPragmas {
		stmt, err := conn.Prepare(q)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("db: prepare %q: %w", q, err)
		}
		_, err = stmt.Exec(nil)
		stmt.Close()
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("db: exec %q: %w", q, err)
		}
	}
	return conn, nil
}

func (pc *pragmaConnector) Driver() driver.Driver { return pc.drv }

// Open opens (or creates) the SQLite database at dbPath.
// SEC-013: Rejects paths containing ".." to prevent directory traversal.
func Open(dbPath string) (*sql.DB, error) {
	cleaned := filepath.Clean(dbPath)
	if strings.Contains(cleaned, "..") {
		return nil, fmt.Errorf("db: path %q contains '..' and is not allowed", dbPath)
	}
	if err := os.MkdirAll(filepath.Dir(cleaned), 0o755); err != nil {
		return nil, fmt.Errorf("db: create directory: %w", err)
	}

	// Open a temporary handle solely to retrieve the underlying driver.
	tmp, err := sql.Open("sqlite", cleaned)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	drv := tmp.Driver()
	tmp.Close()

	db := sql.OpenDB(&pragmaConnector{dsn: cleaned, drv: drv})
	// Allow multiple concurrent readers in WAL mode.
	// SQLite serializes writers at the engine level regardless.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	return db, nil
}
