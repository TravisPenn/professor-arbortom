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
	"strconv"
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
	hint := NormalizeLocation(locationHint)

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
			loc := NormalizeLocation(cols[1])
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

// NormalizeLocation converts both DB slugs ("kanto-route-4") and walkthrough
// display names ("Route 4") to a comparable form.
func NormalizeLocation(s string) string {
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

// tmEntry holds parsed data for one row of the "All TMs" reference table.
type tmEntry struct {
	Number   int
	Location string
	Badge    int // -1 when unparseable ("Varies")
}

// parseTMTable extracts all rows from the "## All TMs" reference table.
// Returns nil when the version has no such table.
func parseTMTable(versionName string) []tmEntry {
	fname, ok := versionFile[strings.ToLower(versionName)]
	if !ok {
		return nil
	}
	data, err := fs.ReadFile(fname)
	if err != nil {
		return nil
	}

	doc := string(data)
	idx := strings.Index(doc, "## All TMs")
	if idx < 0 {
		return nil
	}
	section := doc[idx:]

	var entries []tmEntry
	headerSkipped := 0
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "## All TMs") {
			break
		}
		if !strings.Contains(trimmed, "|") {
			continue
		}
		if headerSkipped < 2 {
			headerSkipped++
			continue
		}
		cols := strings.Split(trimmed, "|")
		if len(cols) < 7 {
			continue
		}
		// Column layout: | TM | Move | Type | Location | Method | Earliest |
		tmCol := strings.TrimSpace(cols[1])
		locationCol := strings.TrimSpace(cols[4])
		earliestCol := strings.TrimSpace(cols[6])

		var tmNum int
		if strings.HasPrefix(strings.ToLower(tmCol), "tm") {
			tmNum, _ = strconv.Atoi(strings.TrimLeft(tmCol[2:], "0"))
		}
		if tmNum == 0 {
			continue
		}

		badge := -1
		if strings.HasPrefix(earliestCol, "Badge ") {
			badge, _ = strconv.Atoi(strings.TrimPrefix(earliestCol, "Badge "))
		}
		entries = append(entries, tmEntry{Number: tmNum, Location: locationCol, Badge: badge})
	}
	return entries
}

// AvailableTMs returns the set of TM numbers obtainable at or before the given
// badge count. Returns nil when the version has no TM reference table.
func AvailableTMs(versionName string, badgeCount int) map[int]bool {
	entries := parseTMTable(versionName)
	if entries == nil {
		return nil
	}
	available := make(map[int]bool)
	for _, e := range entries {
		if e.Badge >= 0 && e.Badge <= badgeCount {
			available[e.Number] = true
		}
	}
	if len(available) == 0 {
		return nil
	}
	return available
}

// TMLocations returns a map of TM number → location name (e.g. "Silph Co. 3F").
// Returns nil when the version has no TM reference table.
func TMLocations(versionName string) map[int]string {
	entries := parseTMTable(versionName)
	if entries == nil {
		return nil
	}
	locs := make(map[int]string, len(entries))
	for _, e := range entries {
		locs[e.Number] = e.Location
	}
	return locs
}
