// Package walkthroughs provides embedded walkthrough guides for each game version.
// Files are segmented by badge (## Badge N: ...) and can be queried by version
// name and badge count to return the relevant progression section.
//
// Sub-section helpers (SectionHeader, SubSection, FilterTableByLocation) allow
// callers to extract targeted slices for LLM context without sending the entire
// badge block.
package walkthroughs

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed *.md
var fs embed.FS

// versionFile maps the game_version.name column value (lowercase, as stored
// in SQLite) to the corresponding embedded walkthrough filename.
var versionFile = map[string]string{
	"firered":   "firered.md",
	"leafgreen": "leafgreen.md",
	"ruby":      "ruby.md",
	"sapphire":  "sapphire.md",
	"emerald":   "emerald.md",
}

// Versions returns every version name that has a walkthrough file.
func Versions() []string {
	out := make([]string, 0, len(versionFile))
	for k := range versionFile {
		out = append(out, k)
	}
	return out
}

// Lookup returns the walkthrough section for the given badge count.
// badgeCount 0 returns the "Starting Out" section, 1-7 return the named badge,
// 8 returns the "Elite Four" section, and 9 returns "Sevii Islands" (post-game).
// Returns empty string when the version has no walkthrough.
func Lookup(versionName string, badgeCount int) string {
	fname, ok := versionFile[strings.ToLower(versionName)]
	if !ok {
		return ""
	}
	data, err := fs.ReadFile(fname)
	if err != nil {
		return ""
	}
	return extractSection(string(data), badgeCount)
}

// Full returns the entire walkthrough text for a version, or empty string.
func Full(versionName string) string {
	fname, ok := versionFile[strings.ToLower(versionName)]
	if !ok {
		return ""
	}
	data, err := fs.ReadFile(fname)
	if err != nil {
		return ""
	}
	return string(data)
}

// SectionHeader returns the content before the first ### sub-section header
// within a badge section. This is typically the gym info, threats, strategy,
// and badge reward — the most important context for any LLM query.
func SectionHeader(section string) string {
	lines := strings.Split(section, "\n")
	var sb strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, "### ") {
			break
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}

// SubSection extracts a single ### sub-section by name from a badge section.
// For example SubSection(section, "Unlocks") returns the ### Unlocks block.
// Returns empty string if the sub-section is not found.
func SubSection(section, name string) string {
	target := "### " + name
	lines := strings.Split(section, "\n")
	var in bool
	var sb strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, "### ") || strings.HasPrefix(line, "## ") {
			if in {
				break
			}
			if strings.HasPrefix(line, target) {
				in = true
				sb.WriteString(line)
				sb.WriteByte('\n')
				continue
			}
		}
		if in {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// FilterTableByLocation filters a markdown table section to only include rows
// whose first column (Location) fuzzy-matches locationHint. locationHint is
// expected as a DB slug like "cerulean-city" or "kanto-route-4". If no rows
// match, the full table is returned so the LLM still has context.
func FilterTableByLocation(tableSection, locationHint string) string {
	if locationHint == "" {
		return tableSection
	}
	hint := normalizeLocation(locationHint)

	lines := strings.Split(tableSection, "\n")
	var header []string
	var matched []string
	var total int

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, "|") {
			continue
		}
		// First two pipe-lines are header row + separator
		if len(header) < 2 {
			header = append(header, line)
			continue
		}
		total++
		cols := strings.SplitN(trimmed, "|", 3)
		if len(cols) >= 2 {
			loc := normalizeLocation(cols[1])
			if strings.Contains(loc, hint) || strings.Contains(hint, loc) {
				matched = append(matched, line)
			}
		}
	}

	// If nothing matched or the table is tiny, return the whole thing.
	if len(matched) == 0 || total <= 3 {
		return tableSection
	}
	return strings.Join(header, "\n") + "\n" + strings.Join(matched, "\n")
}

// normalizeLocation converts both DB slugs ("kanto-route-4") and walkthrough
// display names ("Route 4") to a comparable form.
func normalizeLocation(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "*", "")
	// Strip region prefix from PokeAPI slugs
	s = strings.TrimPrefix(s, "kanto ")
	s = strings.TrimPrefix(s, "hoenn ")
	s = strings.TrimPrefix(s, "johto ")
	return strings.TrimSpace(s)
}

// extractSection pulls the content block for the requested badge.
// Sections are delimited by "## Badge N:", "## Elite Four", or "## Sevii Islands".
func extractSection(doc string, badgeCount int) string {
	var target string
	switch {
	case badgeCount > 8:
		target = "## Sevii Islands"
	case badgeCount == 8:
		target = "## Elite Four"
	default:
		target = fmt.Sprintf("## Badge %d:", badgeCount)
	}

	lines := strings.Split(doc, "\n")
	var inSection bool
	var sb strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if inSection {
				// hit the next section header — stop
				break
			}
			if strings.HasPrefix(line, target) {
				inSection = true
				sb.WriteString(line)
				sb.WriteByte('\n')
				continue
			}
		}
		if inSection {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}
