package db

import (
	"database/sql"
	"fmt"

	"github.com/TravisPenn/professor-arbortom/migrations"
)

const currentVersion = 19

// migrationFiles lists every migration SQL file in application order.
// Adding a new migration requires only appending to this slice and bumping currentVersion.
var migrationFiles = []string{
	"001_initial.sql",
	"002_starters.sql",
	"003_merge_pokemon.sql",
	"004_archive_run.sql",
	"005_static_locations.sql",
	"006_coach_improvements.sql",
	"007_tm_moves.sql",
	"008_hm_tutor_moves.sql",
	"009_pokemon_types_stats.sql",
	"010_current_moves.sql",
	"011_static_encounters.sql",
	"012_pokemon_acquisition.sql",
	"013_merge_pokemon_tables.sql",
	"014_merge_run_progress.sql",
	"015_merge_run_settings.sql",
	"016_drop_legacy_tables.sql",
	"017_opponent_teams.sql",
	"018_fix_run_pokemon_fk.sql",
	"019_move_tooltip_fields.sql",
}

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

	for i, f := range migrationFiles {
		target := i + 1
		if version >= target {
			continue
		}
		if err := applyMigration(db, f); err != nil {
			return err
		}
		if err := setUserVersion(db, target); err != nil {
			return err
		}
	}

	return nil
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
