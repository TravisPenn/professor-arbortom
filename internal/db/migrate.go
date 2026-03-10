package db

import (
	"database/sql"
	"fmt"

	"github.com/pennt/pokemonprofessor/migrations"
)

const currentVersion = 4

// Migrate checks PRAGMA user_version and applies pending migrations.
// Safe to call on every startup.
func Migrate(db *sql.DB) error {
	version, err := userVersion(db)
	if err != nil {
		return err
	}

	if version >= currentVersion {
		return nil // already up-to-date
	}

	if version == 0 {
		if err := applyMigration(db, "001_initial.sql"); err != nil {
			return fmt.Errorf("db: migration 001: %w", err)
		}
		version = 1
	}

	if version == 1 {
		if err := applyMigration(db, "002_starters.sql"); err != nil {
			return fmt.Errorf("db: migration 002: %w", err)
		}
		version = 2
	}

	if version == 2 {
		if err := applyMigration(db, "003_merge_pokemon.sql"); err != nil {
			return fmt.Errorf("db: migration 003: %w", err)
		}
		version = 3
	}

	if version == 3 {
		if err := applyMigration(db, "004_archive_run.sql"); err != nil {
			return fmt.Errorf("db: migration 004: %w", err)
		}
		return setUserVersion(db, 4)
	}

	return fmt.Errorf("db: unknown user_version %d", version)
}

func applyMigration(db *sql.DB, filename string) error {
	data, err := migrations.FS.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("db: read migration %s: %w", filename, err)
	}

	if _, err := db.Exec(string(data)); err != nil {
		return fmt.Errorf("db: exec migration %s: %w", filename, err)
	}
	return nil
}

func userVersion(db *sql.DB) (int, error) {
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return 0, fmt.Errorf("db: read user_version: %w", err)
	}
	return v, nil
}

func setUserVersion(db *sql.DB, v int) error {
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", v)); err != nil {
		return fmt.Errorf("db: set user_version %d: %w", v, err)
	}
	return nil
}
