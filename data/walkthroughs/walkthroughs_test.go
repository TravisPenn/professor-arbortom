package walkthroughs

import (
	"strings"
	"testing"
)

func TestLookup_FireRed_Badge0(t *testing.T) {
	section := Lookup("firered", 0)
	if section == "" {
		t.Fatal("expected non-empty walkthrough for firered badge 0")
	}
	if !strings.Contains(section, "Badge 0") {
		t.Error("section should contain 'Badge 0' header")
	}
	if !strings.Contains(section, "Starter") {
		t.Error("section should mention Starter")
	}
	// Must NOT contain Badge 1 content (next section)
	if strings.Contains(section, "## Badge 1") {
		t.Error("section should not include Badge 1 header")
	}
}

func TestLookup_Emerald_Badge8(t *testing.T) {
	section := Lookup("emerald", 8)
	if section == "" {
		t.Fatal("expected non-empty walkthrough for emerald badge 8 (Elite Four)")
	}
	if !strings.Contains(section, "Elite Four") {
		t.Error("badge 8 should return Elite Four section")
	}
}

func TestLookup_Sevii(t *testing.T) {
	section := Lookup("firered", 9)
	if section == "" {
		t.Fatal("expected non-empty walkthrough for firered badge > 8 (Sevii Islands)")
	}
	if !strings.Contains(section, "Sevii Islands") {
		t.Error("badge 9 should return Sevii Islands section")
	}
}

func TestLookup_UnknownVersion(t *testing.T) {
	section := Lookup("gold", 0)
	if section != "" {
		t.Errorf("expected empty for unknown version, got %q", section)
	}
}

func TestLookup_AllVersions(t *testing.T) {
	for _, v := range Versions() {
		section := Lookup(v, 0)
		if section == "" {
			t.Errorf("version %q has no badge 0 section", v)
		}
	}
}

func TestLookup_CaseInsensitive(t *testing.T) {
	section := Lookup("FireRed", 3)
	if section == "" {
		t.Fatal("Lookup should be case-insensitive")
	}
	if !strings.Contains(section, "Badge 3") {
		t.Error("should contain Badge 3 content")
	}
}

func TestFull(t *testing.T) {
	full := Full("ruby")
	if full == "" {
		t.Fatal("Full should return entire walkthrough")
	}
	if !strings.Contains(full, "Badge 0") || !strings.Contains(full, "Elite Four") {
		t.Error("Full should contain both Badge 0 and Elite Four sections")
	}
}

func TestSectionHeader(t *testing.T) {
	section := Lookup("firered", 2)
	hdr := SectionHeader(section)
	if hdr == "" {
		t.Fatal("SectionHeader should return gym info")
	}
	if !strings.Contains(hdr, "Misty") {
		t.Error("Badge 2 header should mention Misty")
	}
	// Header must not include sub-sections
	if strings.Contains(hdr, "### ") {
		t.Error("SectionHeader should stop before ### sub-sections")
	}
}

func TestSubSection(t *testing.T) {
	section := Lookup("firered", 3)

	unlocks := SubSection(section, "Unlocks")
	if unlocks == "" {
		t.Fatal("SubSection('Unlocks') should not be empty for badge 3")
	}
	if !strings.Contains(unlocks, "HM01 Cut") {
		t.Error("Badge 3 Unlocks should mention HM01 Cut")
	}

	items := SubSection(section, "Items & TMs")
	if items == "" {
		t.Fatal("SubSection('Items & TMs') should not be empty for badge 3")
	}
	if !strings.Contains(items, "S.S. Anne") {
		t.Error("Badge 3 Items should mention S.S. Anne")
	}

	// Non-existent sub-section
	if SubSection(section, "Nonexistent") != "" {
		t.Error("non-existent sub-section should return empty")
	}
}

func TestFilterTableByLocation(t *testing.T) {
	section := Lookup("firered", 4)
	items := SubSection(section, "Items & TMs")
	if items == "" {
		t.Fatal("need Items & TMs for badge 4")
	}

	// Filter to Celadon content using DB-style slug
	filtered := FilterTableByLocation(items, "celadon-city")
	if !strings.Contains(filtered, "Celadon") {
		t.Error("filtered result should contain Celadon entries")
	}

	// Empty location hint returns full table
	full := FilterTableByLocation(items, "")
	if full != items {
		t.Error("empty location hint should return full table")
	}

	// A location not in the table should fall back to full table
	fallback := FilterTableByLocation(items, "some-unknown-place")
	if fallback != items {
		t.Error("unmatched location should fall back to full table")
	}
}

func TestNormalizeLocation(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"kanto-route-4", "route 4"},
		{"cerulean-city", "cerulean city"},
		{"Cerulean City", "cerulean city"},
		{"Mt. Moon", "mt. moon"},
		{"  hoenn-route-110 ", "route 110"},
	}
	for _, tc := range tests {
		got := normalizeLocation(tc.input)
		if got != tc.expected {
			t.Errorf("normalizeLocation(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
