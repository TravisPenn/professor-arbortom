// Package migrations exposes the embedded SQL migration files.
package migrations

import "embed"

// FS holds all SQL migration files.
//go:embed *.sql
var FS embed.FS
