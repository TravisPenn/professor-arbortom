// Package static exposes embedded CSS and asset files.
package static

import "embed"

// FS holds all static asset files.
//go:embed style.css runs.js routes.js coach.js thinking.mp4
var FS embed.FS
