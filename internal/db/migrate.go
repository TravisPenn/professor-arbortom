package db

import (
	"database/sql"
	"fmt"

	"github.com/TravisPenn/professor-arbortom/migrations"
)

const currentVersion = 15

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
		version = 4
	}

	if version == 4 {
		if err := applyMigration(db, "005_static_locations.sql"); err != nil {
			return fmt.Errorf("db: migration 005: %w", err)
		}
		version = 5
	}

	if version == 5 {
		if err := applyMigration(db, "006_coach_improvements.sql"); err != nil {
			return fmt.Errorf("db: migration 006: %w", err)
		}
		if err := setUserVersion(db, 6); err != nil {
			return err
		}
		version = 6
	}

	if version == 6 {
		if err := applyMigration(db, "007_tm_moves.sql"); err != nil {
			return fmt.Errorf("db: migration 007: %w", err)
		}
		if err := setUserVersion(db, 7); err != nil {
			return err
		}
		version = 7
	}

	if version == 7 {
		if err := applyMigration(db, "008_hm_tutor_moves.sql"); err != nil {
			return fmt.Errorf("db: migration 008: %w", err)
		}
		if err := setUserVersion(db, 8); err != nil {
			return err
		}
		version = 8
	}

	if version == 8 {
		if err := applyMigration(db, "009_pokemon_types_stats.sql"); err != nil {
			return fmt.Errorf("db: migration 009: %w", err)
		}
		if err := setUserVersion(db, 9); err != nil {
			return err
		}
		version = 9
	}

	if version == 9 {
		if err := applyMigration(db, "010_current_moves.sql"); err != nil {
			return fmt.Errorf("db: migration 010: %w", err)
		}
		if err := setUserVersion(db, 10); err != nil {
			return err
		}
		version = 10
	}

	if version == 10 {
		if err := applyMigration(db, "011_static_encounters.sql"); err != nil {
			return fmt.Errorf("db: migration 011: %w", err)
		}
		if err := setUserVersion(db, 11); err != nil {
			return err
		}
		version = 11
	}

	if version == 11 {
		if err := applyMigration(db, "012_pokemon_acquisition.sql"); err != nil {
			return fmt.Errorf("db: migration 012: %w", err)
		}
		if err := setUserVersion(db, 12); err != nil {
			return err
		}
		version = 12
	}

	if version == 12 {
		if err := applyMigration(db, "013_merge_pokemon_tables.sql"); err != nil {
			return fmt.Errorf("db: migration 013: %w", err)
		}
		if err := setUserVersion(db, 13); err != nil {
			return err
		}
		version = 13
	}

	if version == 13 {
		if err := applyMigration(db, "014_merge_run_progress.sql"); err != nil {
			return fmt.Errorf("db: migration 014: %w", err)
		}
		if err := setUserVersion(db, 14); err != nil {
			return err
		}
		version = 14
	}

	if version == 14 {
		if err := applyMigration(db, "015_merge_run_settings.sql"); err != nil {
			return fmt.Errorf("db: migration 015: %w", err)
		}
		return setUserVersion(db, 15)
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

// setUserVersion sets the SQLite user_version pragma.
// SEC-004: PRAGMAs cannot use parameterized queries. We use Sprintf with %d
// (integer-only) and bound-check to prevent misuse if the function signature
// ever changes.
func setUserVersion(db *sql.DB, v int) error {
	if v < 0 || v > 1000 {
		return fmt.Errorf("db: user_version %d out of allowed range 0–1000", v)
	}
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", v)); err != nil {
		return fmt.Errorf("db: set user_version %d: %w", v, err)
	}
	return nil
}
