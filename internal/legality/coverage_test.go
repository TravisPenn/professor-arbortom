package legality

import (
	"testing"
)

// allWaterTeam returns a team of 3 Water-type Pokémon with no moves,
// used to verify Ground and Electric show as weaknesses.
func allWaterTeam() []TeamMember {
	return []TeamMember{
		{SpeciesName: "squirtle", Type1: TypeWater},
		{SpeciesName: "poliwag", Type1: TypeWater},
		{SpeciesName: "horsea", Type1: TypeWater},
	}
}

func TestTeamDefensiveProfile_WaterTeamWeaknesses(t *testing.T) {
	team := allWaterTeam()
	profile := TeamDefensiveProfile(team)

	weakSet := make(map[string]bool)
	for _, w := range profile.Weaknesses {
		weakSet[w.Type] = true
	}

	if !weakSet[TypeElectric] {
		t.Error("water team should be weak to electric")
	}
	if !weakSet[TypeGrass] {
		t.Error("water team should be weak to grass")
	}
	// Fire, Ice, Steel, Water should NOT be weaknesses (water resists or is neutral)
	if weakSet[TypeFire] {
		t.Error("water team should not be weak to fire")
	}
}

func TestTeamDefensiveProfile_Immunities(t *testing.T) {
	// Ghost/Normal member: Normal and Fighting are immune targets.
	team := []TeamMember{
		{SpeciesName: "sableye", Type1: TypeDark, Type2: TypeGhost},
	}
	profile := TeamDefensiveProfile(team)

	immuneSet := make(map[string]bool)
	for _, im := range profile.Immunities {
		immuneSet[im] = true
	}
	if !immuneSet[TypeNormal] {
		t.Error("dark/ghost sableye should be immune to normal")
	}
	if !immuneSet[TypeFighting] {
		t.Error("dark/ghost sableye should be immune to fighting")
	}
	if !immuneSet[TypePsychic] {
		t.Error("dark/ghost sableye should be immune to psychic")
	}
}

func TestTeamOffensiveCoverage_NoFightingMoves(t *testing.T) {
	// Team with only water moves — should have gaps for types Fighting can hit.
	team := []TeamMember{
		{SpeciesName: "squirtle", Type1: TypeWater, MoveTypes: []string{TypeWater}},
	}
	cov := TeamOffensiveCoverage(team)

	gapSet := make(map[string]bool)
	for _, g := range cov.Gaps {
		gapSet[g] = true
	}

	// Water is super-effective vs Fire, Ground, Rock — those should be covered.
	covSet := make(map[string]bool)
	for _, c := range cov.Covered {
		covSet[c.DefendingType] = true
	}
	if !covSet[TypeFire] {
		t.Error("water move should cover fire type")
	}
	if !covSet[TypeGround] {
		t.Error("water move should cover ground type")
	}
	if !covSet[TypeRock] {
		t.Error("water move should cover rock type")
	}

	// Normal type is not hit SE by water.
	if !gapSet[TypeNormal] {
		t.Error("water-only coverage should have a gap for normal")
	}
}

func TestTeamOffensiveCoverage_DedupCovers(t *testing.T) {
	team := []TeamMember{
		{SpeciesName: "charizard", Type1: TypeFire, Type2: TypeFlying, MoveTypes: []string{TypeFire, TypeFire}},
	}
	cov := TeamOffensiveCoverage(team)
	for _, c := range cov.Covered {
		if c.DefendingType == TypeGrass {
			if len(c.CoveredBy) != 1 {
				t.Errorf("duplicate fire entries for grass: got %d, want 1", len(c.CoveredBy))
			}
		}
	}
}

func TestCounterChain_Electric(t *testing.T) {
	team := allWaterTeam()
	// Give team water moves only.
	for i := range team {
		team[i].MoveTypes = []string{TypeWater}
	}
	hops := CounterChain(TypeElectric, team, 2)
	if len(hops) < 1 {
		t.Fatal("expected at least 1 hop")
	}
	// Depth 1: types SE against Electric → Ground
	hop1 := hops[0]
	if hop1.Depth != 1 {
		t.Errorf("hop1.Depth = %d, want 1", hop1.Depth)
	}
	foundGround := false
	for _, ct := range hop1.CounterTypes {
		if ct == TypeGround {
			foundGround = true
		}
	}
	if !foundGround {
		t.Error("depth-1 counter to electric should include ground")
	}

	// Team has only Water moves — shouldn't cover Ground at depth 1.
	if hop1.TeamCovers {
		t.Error("water-only team should not cover ground (depth 1 counter to electric)")
	}

	if len(hops) >= 2 {
		hop2 := hops[1]
		// Depth 2 counter to Ground → Water, Ice, Grass
		foundWater := false
		for _, ct := range hop2.CounterTypes {
			if ct == TypeWater {
				foundWater = true
			}
		}
		if !foundWater {
			t.Error("depth-2 counter should include water")
		}
		// Team has water moves → should cover depth 2.
		if !hop2.TeamCovers {
			t.Error("depth-2 counter should be covered by water team's water moves")
		}
	}
}

func TestCounterChain_MaxDepthClamped(t *testing.T) {
	team := allWaterTeam()
	hops := CounterChain(TypeFire, team, 10)
	if len(hops) > 3 {
		t.Errorf("CounterChain returned %d hops, want ≤3", len(hops))
	}
}

func TestMemberEffectiveness_DualType(t *testing.T) {
	// Fire/Flying vs Ground should be 0.0 (flying immune) × ... wait:
	// Ground vs Fire = 2.0, Ground vs Flying = 0.0 → 0.0
	m := TeamMember{SpeciesName: "charizard", Type1: TypeFire, Type2: TypeFlying}
	mult := memberEffectiveness(TypeGround, m)
	if !approxEqual(mult, 0.0) {
		t.Errorf("ground vs fire/flying = %v, want 0.0", mult)
	}
}

func TestMemberEffectiveness_MonoType(t *testing.T) {
	m := TeamMember{SpeciesName: "charmander", Type1: TypeFire}
	mult := memberEffectiveness(TypeWater, m)
	if !approxEqual(mult, 2.0) {
		t.Errorf("water vs fire = %v, want 2.0", mult)
	}
}
