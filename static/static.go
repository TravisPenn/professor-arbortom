// Package static exposes embedded CSS and asset files.
package static

import "embed"

// FS holds all static asset files.
//go:embed style.css runs.js routes.js coach.js
var FS embed.FS
