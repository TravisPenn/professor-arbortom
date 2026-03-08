package legality

import (
	"math"
	"testing"
)

const delta = 1e-9

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < delta
}

func TestTypeEffectiveness_SuperEffective(t *testing.T) {
	tests := []struct {
		atk, def string
		want     float64
	}{
		{"fire", "grass", 2.0},
		{"fire", "ice", 2.0},
		{"fire", "bug", 2.0},
		{"fire", "steel", 2.0},
		{"water", "fire", 2.0},
		{"water", "ground", 2.0},
		{"water", "rock", 2.0},
		{"electric", "water", 2.0},
		{"electric", "flying", 2.0},
		{"grass", "water", 2.0},
		{"grass", "ground", 2.0},
		{"grass", "rock", 2.0},
		{"ice", "grass", 2.0},
		{"ice", "ground", 2.0},
		{"ice", "flying", 2.0},
		{"ice", "dragon", 2.0},
		{"fighting", "normal", 2.0},
		{"fighting", "ice", 2.0},
		{"fighting", "rock", 2.0},
		{"fighting", "dark", 2.0},
		{"fighting", "steel", 2.0},
		{"poison", "grass", 2.0},
		{"poison", "fairy", 2.0},
		{"ground", "fire", 2.0},
		{"ground", "electric", 2.0},
		{"ground", "poison", 2.0},
		{"ground", "rock", 2.0},
		{"ground", "steel", 2.0},
		{"flying", "grass", 2.0},
		{"flying", "fighting", 2.0},
		{"flying", "bug", 2.0},
		{"psychic", "fighting", 2.0},
		{"psychic", "poison", 2.0},
		{"bug", "grass", 2.0},
		{"bug", "psychic", 2.0},
		{"bug", "dark", 2.0},
		{"rock", "fire", 2.0},
		{"rock", "ice", 2.0},
		{"rock", "flying", 2.0},
		{"rock", "bug", 2.0},
		{"ghost", "psychic", 2.0},
		{"ghost", "ghost", 2.0},
		{"dragon", "dragon", 2.0},
		{"dark", "psychic", 2.0},
		{"dark", "ghost", 2.0},
		{"steel", "ice", 2.0},
		{"steel", "rock", 2.0},
		{"steel", "fairy", 2.0},
		{"fairy", "fighting", 2.0},
		{"fairy", "dragon", 2.0},
		{"fairy", "dark", 2.0},
	}

	for _, tc := range tests {
		got := TypeEffectiveness(tc.atk, tc.def)
		if !approxEqual(got, tc.want) {
			t.Errorf("TypeEffectiveness(%q, %q) = %v, want %v", tc.atk, tc.def, got, tc.want)
		}
	}
}

func TestTypeEffectiveness_NotVeryEffective(t *testing.T) {
	tests := []struct {
		atk, def string
	}{
		{"normal", "rock"},
		{"normal", "steel"},
		{"fire", "fire"},
		{"fire", "water"},
		{"fire", "rock"},
		{"fire", "dragon"},
		{"water", "water"},
		{"water", "grass"},
		{"water", "dragon"},
		{"electric", "electric"},
		{"electric", "grass"},
		{"electric", "dragon"},
		{"grass", "fire"},
		{"grass", "grass"},
		{"grass", "poison"},
		{"grass", "flying"},
		{"grass", "bug"},
		{"grass", "dragon"},
		{"grass", "steel"},
		{"ice", "fire"},
		{"ice", "water"},
		{"ice", "ice"},
		{"ice", "steel"},
		{"fighting", "poison"},
		{"fighting", "flying"},
		{"fighting", "psychic"},
		{"fighting", "bug"},
		{"fighting", "fairy"},
		{"poison", "poison"},
		{"poison", "ground"},
		{"poison", "rock"},
		{"poison", "ghost"},
		{"ground", "grass"},
		{"ground", "bug"},
		{"flying", "electric"},
		{"flying", "rock"},
		{"flying", "steel"},
		{"psychic", "psychic"},
		{"psychic", "steel"},
		{"bug", "fire"},
		{"bug", "fighting"},
		{"bug", "poison"},
		{"bug", "flying"},
		{"bug", "ghost"},
		{"bug", "steel"},
		{"bug", "fairy"},
		{"rock", "fighting"},
		{"rock", "ground"},
		{"rock", "steel"},
		{"ghost", "dark"},
		{"dragon", "steel"},
		{"dark", "fighting"},
		{"dark", "dark"},
		{"dark", "fairy"},
		{"steel", "fire"},
		{"steel", "water"},
		{"steel", "electric"},
		{"steel", "steel"},
		{"fairy", "fire"},
		{"fairy", "poison"},
		{"fairy", "steel"},
	}
	for _, tc := range tests {
		got := TypeEffectiveness(tc.atk, tc.def)
		if !approxEqual(got, 0.5) {
			t.Errorf("TypeEffectiveness(%q, %q) = %v, want 0.5", tc.atk, tc.def, got)
		}
	}
}

func TestTypeEffectiveness_Immune(t *testing.T) {
	tests := []struct {
		atk, def string
	}{
		{"normal", "ghost"},
		{"electric", "ground"},
		{"fighting", "ghost"},
		{"poison", "steel"},
		{"ground", "flying"},
		{"psychic", "dark"},
		{"ghost", "normal"},
		{"dragon", "fairy"},
	}
	for _, tc := range tests {
		got := TypeEffectiveness(tc.atk, tc.def)
		if !approxEqual(got, 0.0) {
			t.Errorf("TypeEffectiveness(%q, %q) = %v, want 0.0 (immune)", tc.atk, tc.def, got)
		}
	}
}

func TestTypeEffectiveness_Neutral(t *testing.T) {
	if got := TypeEffectiveness("normal", "normal"); !approxEqual(got, 1.0) {
		t.Errorf("normal vs normal = %v, want 1.0", got)
	}
	if got := TypeEffectiveness("water", "electric"); !approxEqual(got, 1.0) {
		t.Errorf("water vs electric = %v, want 1.0", got)
	}
}

func TestTypeEffectiveness_UnknownTypes(t *testing.T) {
	if got := TypeEffectiveness("unknown", "fire"); !approxEqual(got, 1.0) {
		t.Errorf("unknown atk type should be 1.0, got %v", got)
	}
	if got := TypeEffectiveness("fire", "unknown"); !approxEqual(got, 1.0) {
		t.Errorf("unknown def type should be 1.0, got %v", got)
	}
}

func TestDualTypeEffectiveness(t *testing.T) {
	tests := []struct {
		atk, def1, def2 string
		want            float64
	}{
		// Ground vs Fire/Flying: 2.0 (fire) × 0.0 (flying) = 0.0
		{"ground", "fire", "flying", 0.0},
		// Electric vs Water/Flying: 2.0 × 2.0 = 4.0
		{"electric", "water", "flying", 4.0},
		// Fire vs Grass/Steel: 2.0 × 2.0 = 4.0
		{"fire", "grass", "steel", 4.0},
		// Water vs Fire/Rock: 2.0 × 2.0 = 4.0
		{"water", "fire", "rock", 4.0},
		// Electric vs Grass/Ground: 0.5 × 0.0 = 0.0
		{"electric", "grass", "ground", 0.0},
		// Ice vs Dragon/Flying: 2.0 × 2.0 = 4.0
		{"ice", "dragon", "flying", 4.0},
		// Normal vs Ghost/Normal: 0.0 × 1.0 = 0.0
		{"normal", "ghost", "normal", 0.0},
		// Fighting vs Normal/Flying: 2.0 × 0.5 = 1.0 (cancels)
		{"fighting", "normal", "flying", 1.0},
	}

	for _, tc := range tests {
		got := DualTypeEffectiveness(tc.atk, tc.def1, tc.def2)
		if !approxEqual(got, tc.want) {
			t.Errorf("DualTypeEffectiveness(%q, %q, %q) = %v, want %v",
				tc.atk, tc.def1, tc.def2, got, tc.want)
		}
	}
}

func TestAllTypesCount(t *testing.T) {
	if len(AllTypes) != 18 {
		t.Errorf("AllTypes has %d entries, want 18", len(AllTypes))
	}
}

func TestAllTypesInEffectivenessMap(t *testing.T) {
	// Every type must appear as an attacking type in the effectiveness map
	// (even if most matchups are neutral, the map key should exist for the
	// non-neutral ones). We verify by ensuring none of the type constants
	// are misspelled relative to AllTypes.
	typeSet := make(map[string]bool, len(AllTypes))
	for _, tp := range AllTypes {
		typeSet[tp] = true
	}
	for atk := range effectiveness {
		if !typeSet[atk] {
			t.Errorf("effectiveness map contains unknown attacking type %q", atk)
		}
		for def := range effectiveness[atk] {
			if !typeSet[def] {
				t.Errorf("effectiveness map contains unknown defending type %q under attack type %q", def, atk)
			}
		}
	}
}
