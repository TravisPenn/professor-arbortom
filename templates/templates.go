// Package templates exposes embedded HTML template files.
package templates

import "embed"

// FS holds all HTML template files.
//go:embed *.html
var FS embed.FS
