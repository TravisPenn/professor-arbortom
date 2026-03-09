package handlers

import "github.com/pennt/pokemonprofessor/internal/legality"

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
		LocationName:  a.LocationName,
		BlockedByRule: b,
	}
}

// moveToOption converts a legality.Move to a MoveOption.
func moveToOption(m legality.Move) MoveOption {
	return MoveOption{
		ID:          m.MoveID,
		Name:        m.Name,
		TypeName:    m.TypeName,
		LearnMethod: m.LearnMethod,
		Level:       m.LevelLearned,
	}
}

// itemToOption converts a legality.Item to an ItemOption.
func itemToOption(i legality.Item) ItemOption {
	return ItemOption{
		ID:       i.ItemID,
		Name:     i.Name,
		Category: i.Category,
		Source:   i.Source,
	}
}
