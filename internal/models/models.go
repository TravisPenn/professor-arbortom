// Package models contains shared database-backed model types used across handlers.
package models

// Run represents a player's playthrough record.
type Run struct {
	ID        int
	Name      string
	UserID    int
	VersionID int
	CreatedAt string
}

// RunProgress holds the current progress state within a run.
type RunProgress struct {
	RunID             int
	CurrentLocationID *int
	BadgeCount        int
	UpdatedAt         string
}

// ActiveRule is a resolved rule state for a run.
type ActiveRule struct {
	Key        string
	Enabled    bool
	ParamsJSON string
}

// GameVersion describes a Gen 3 game version.
type GameVersion struct {
	ID             int
	Name           string
	VersionGroupID int
	GenerationID   int
}

// Location is a named place in a game version.
type Location struct {
	ID        int
	Name      string
	VersionID int
	Region    string
}

// RuleDef is the catalogue definition of a rule.
type RuleDef struct {
	ID          int
	Key         string
	Description string
}
