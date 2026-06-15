package handlers

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/TravisPenn/professor-arbortom/internal/legality"
)

// acquisitionToFormOption converts a legality.Acquisition to a FormOption.
func acquisitionToFormOption(a legality.Acquisition) FormOption {
	b := ""
	if a.BlockedByRule != nil {
		b = *a.BlockedByRule
	}
	return FormOption{
		ID:            a.FormID,
		SpeciesName:   a.SpeciesName,
		FormName:      a.FormName,
		LocationName:  humanizeLocationName(a.LocationName),
		BlockedByRule: b,
	}
}

// moveToOption converts a legality.Move to a MoveOption.
func moveToOption(m legality.Move) MoveOption {
	return MoveOption{
		ID:            m.MoveID,
		Name:          m.Name,
		TypeName:      m.TypeName,
		LearnMethod:   m.LearnMethod,
		Level:         m.LevelLearned,
		EvoNote:       m.EvoNote,
		TMNumber:      m.TMNumber,
		HMNumber:      m.HMNumber,
		TutorLocation: m.TutorLocation,
		Power:         m.Power,
		Accuracy:      m.Accuracy,
		PP:            m.PP,
		DamageClass:   m.DamageClass,
		Effect:        m.Effect,
	}
}

// itemToOption converts a legality.Item to an ItemOption.
func itemToOption(i legality.Item) ItemOption {
	return ItemOption{
		ID:          i.ItemID,
		Name:        i.Name,
		DisplayName: tmDisplayName(i.Name, i.MoveName),
		Category:    i.Category,
		Source:      i.Source,
		Price:       i.Price,
	}
}

// tmDisplayName returns a display-friendly label for an item name.
// Items matching the PokeAPI TM slug pattern (tm01, tm29 …) become "TM01", "TM29", etc.
// If moveName is non-empty the move name is appended: "TM24 — Thunderbolt".
// All other names are returned as-is.
func tmDisplayName(name, moveName string) string {
	lower := strings.ToLower(name)
	if !strings.HasPrefix(lower, "tm") {
		return name
	}
	suffix := lower[2:]
	if len(suffix) == 0 {
		return name
	}
	for _, r := range suffix {
		if !unicode.IsDigit(r) {
			return name
		}
	}
	dn := fmt.Sprintf("TM%s", suffix)
	if moveName == "" {
		return dn
	}
	return dn + " — " + moveSlugToTitle(moveName)
}

// moveSlugToTitle converts a PokeAPI move slug to a title-cased display name.
// e.g. "ice-beam" → "Ice Beam", "thunderbolt" → "Thunderbolt".
func moveSlugToTitle(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
