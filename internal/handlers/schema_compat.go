package handlers

import "database/sql"

// tableExists reports whether a table is present in the connected SQLite DB.
func tableExists(db *sql.DB, tableName string) bool {
	var n int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		tableName,
	).Scan(&n)
	return err == nil && n > 0
}

// columnExists reports whether a column exists on a table in SQLite.
func columnExists(db *sql.DB, tableName, columnName string) bool {
	rows, err := db.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == columnName {
			return true
		}
	}
	return false
}
