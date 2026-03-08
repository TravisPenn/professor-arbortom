// Package static exposes embedded CSS and asset files.
package static

import "embed"

// FS holds all static asset files.
//go:embed style.css
var FS embed.FS
