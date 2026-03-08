package legality

// Type name constants — Generation 3+ (18 types including Fairy is Gen 6, but
// the chart below covers Gen 3 through Gen 9 by including all 18 types).
const (
	TypeNormal   = "normal"
	TypeFire     = "fire"
	TypeWater    = "water"
	TypeElectric = "electric"
	TypeGrass    = "grass"
	TypeIce      = "ice"
	TypeFighting = "fighting"
	TypePoison   = "poison"
	TypeGround   = "ground"
	TypeFlying   = "flying"
	TypePsychic  = "psychic"
	TypeBug      = "bug"
	TypeRock     = "rock"
	TypeGhost    = "ghost"
	TypeDragon   = "dragon"
	TypeDark     = "dark"
	TypeSteel    = "steel"
	TypeFairy    = "fairy"
)

// AllTypes is the canonical list of all 18 Pokémon types.
var AllTypes = []string{
	TypeNormal, TypeFire, TypeWater, TypeElectric, TypeGrass,
	TypeIce, TypeFighting, TypePoison, TypeGround, TypeFlying,
	TypePsychic, TypeBug, TypeRock, TypeGhost, TypeDragon,
	TypeDark, TypeSteel, TypeFairy,
}

// effectiveness encodes the full 18×18 type chart.
// Outer key = attacking type; inner key = defending type.
// Omitted entries are 1.0 (neutral). Values: 2.0, 0.5, 0.0.
// Source: Bulbapedia Gen 3+ chart (Fairy added Gen 6).
var effectiveness = map[string]map[string]float64{
	TypeNormal: {
		TypeRock:  0.5,
		TypeGhost: 0.0,
		TypeSteel: 0.5,
	},
	TypeFire: {
		TypeFire:   0.5,
		TypeWater:  0.5,
		TypeGrass:  2.0,
		TypeIce:    2.0,
		TypeBug:    2.0,
		TypeRock:   0.5,
		TypeDragon: 0.5,
		TypeSteel:  2.0,
	},
	TypeWater: {
		TypeFire:   2.0,
		TypeWater:  0.5,
		TypeGrass:  0.5,
		TypeGround: 2.0,
		TypeRock:   2.0,
		TypeDragon: 0.5,
	},
	TypeElectric: {
		TypeWater:    2.0,
		TypeElectric: 0.5,
		TypeGrass:    0.5,
		TypeGround:   0.0,
		TypeFlying:   2.0,
		TypeDragon:   0.5,
	},
	TypeGrass: {
		TypeFire:   0.5,
		TypeWater:  2.0,
		TypeGrass:  0.5,
		TypePoison: 0.5,
		TypeGround: 2.0,
		TypeFlying: 0.5,
		TypeBug:    0.5,
		TypeRock:   2.0,
		TypeDragon: 0.5,
		TypeSteel:  0.5,
	},
	TypeIce: {
		TypeFire:   0.5,
		TypeWater:  0.5,
		TypeGrass:  2.0,
		TypeIce:    0.5,
		TypeGround: 2.0,
		TypeFlying: 2.0,
		TypeDragon: 2.0,
		TypeSteel:  0.5,
	},
	TypeFighting: {
		TypeNormal:  2.0,
		TypeIce:     2.0,
		TypePoison:  0.5,
		TypeFlying:  0.5,
		TypePsychic: 0.5,
		TypeBug:     0.5,
		TypeRock:    2.0,
		TypeGhost:   0.0,
		TypeDark:    2.0,
		TypeSteel:   2.0,
		TypeFairy:   0.5,
	},
	TypePoison: {
		TypeGrass:  2.0,
		TypePoison: 0.5,
		TypeGround: 0.5,
		TypeRock:   0.5,
		TypeGhost:  0.5,
		TypeSteel:  0.0,
		TypeFairy:  2.0,
	},
	TypeGround: {
		TypeFire:     2.0,
		TypeElectric: 2.0,
		TypeGrass:    0.5,
		TypePoison:   2.0,
		TypeFlying:   0.0,
		TypeBug:      0.5,
		TypeRock:     2.0,
		TypeSteel:    2.0,
	},
	TypeFlying: {
		TypeElectric: 0.5,
		TypeGrass:    2.0,
		TypeFighting: 2.0,
		TypeBug:      2.0,
		TypeRock:     0.5,
		TypeSteel:    0.5,
	},
	TypePsychic: {
		TypeFighting: 2.0,
		TypePoison:   2.0,
		TypePsychic:  0.5,
		TypeDark:     0.0,
		TypeSteel:    0.5,
	},
	TypeBug: {
		TypeFire:     0.5,
		TypeGrass:    2.0,
		TypeFighting: 0.5,
		TypePoison:   0.5,
		TypeFlying:   0.5,
		TypePsychic:  2.0,
		TypeGhost:    0.5,
		TypeDark:     2.0,
		TypeSteel:    0.5,
		TypeFairy:    0.5,
	},
	TypeRock: {
		TypeFire:     2.0,
		TypeIce:      2.0,
		TypeFighting: 0.5,
		TypeGround:   0.5,
		TypeFlying:   2.0,
		TypeBug:      2.0,
		TypeSteel:    0.5,
	},
	TypeGhost: {
		TypeNormal:  0.0,
		TypePsychic: 2.0,
		TypeGhost:   2.0,
		TypeDark:    0.5,
	},
	TypeDragon: {
		TypeDragon: 2.0,
		TypeSteel:  0.5,
		TypeFairy:  0.0,
	},
	TypeDark: {
		TypeFighting: 0.5,
		TypePsychic:  2.0,
		TypeGhost:    2.0,
		TypeDark:     0.5,
		TypeFairy:    0.5,
	},
	TypeSteel: {
		TypeFire:     0.5,
		TypeWater:    0.5,
		TypeElectric: 0.5,
		TypeIce:      2.0,
		TypeRock:     2.0,
		TypeSteel:    0.5,
		TypeFairy:    2.0,
	},
	TypeFairy: {
		TypeFire:     0.5,
		TypeFighting: 2.0,
		TypePoison:   0.5,
		TypeDragon:   2.0,
		TypeDark:     2.0,
		TypeSteel:    0.5,
	},
}

// TypeEffectiveness returns the damage multiplier when attackType hits a
// single defending type. Returns 1.0 for unknown type names.
func TypeEffectiveness(attackType, defendType string) float64 {
	if inner, ok := effectiveness[attackType]; ok {
		if v, ok := inner[defendType]; ok {
			return v
		}
	}
	return 1.0
}

// DualTypeEffectiveness returns the combined damage multiplier when
// attackType hits a Pokémon with two defending types. The result is the
// product of the individual effectiveness values.
func DualTypeEffectiveness(attackType, defendType1, defendType2 string) float64 {
	return TypeEffectiveness(attackType, defendType1) * TypeEffectiveness(attackType, defendType2)
}
