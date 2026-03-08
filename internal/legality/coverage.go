package legality

import "sort"

// TeamMember holds the data needed for coverage analysis of one party member.
type TeamMember struct {
	SpeciesName string
	Type1       string
	Type2       string   // empty string if mono-type
	MoveTypes   []string // types of known moves
}

// TypeThreat describes an attacking type that threatens the team.
type TypeThreat struct {
	Type           string   `json:"type"`
	WeakMembers    []string `json:"weak_members"`    // species hit for ≥2×
	BestResistance float64  `json:"best_resistance"` // lowest multiplier any member has (e.g. 0.0, 0.25, 0.5)
}

// DefensiveProfile summarises the team's defensive coverage.
type DefensiveProfile struct {
	// Weaknesses: types that hit at least one member for ≥2× with no member resisting.
	Weaknesses []TypeThreat `json:"weaknesses"`
	// Resistances: types that no member is weak to, and at least one resists (≤0.5×).
	Resistances []string `json:"resistances"`
	// Immunities: types that at least one member is fully immune to (0×).
	Immunities []string `json:"immunities"`
	// Uncovered: types where no team member has a super-effective move.
	Uncovered []string `json:"uncovered"`
}

// CoverageEntry records which team members can hit a defending type super-effectively.
type CoverageEntry struct {
	DefendingType string   `json:"defending_type"`
	CoveredBy     []string `json:"covered_by"` // "Charizard (Fire Blast)" format
}

// OffensiveCoverage summarises the team's offensive move coverage.
type OffensiveCoverage struct {
	Covered []CoverageEntry `json:"covered"`
	Gaps    []string        `json:"gaps"`
}

// CounterHop is one depth level in a counter-chain analysis.
type CounterHop struct {
	Depth        int      `json:"depth"`
	CounterTypes []string `json:"counter_types"`
	TeamCovers   bool     `json:"team_covers"`
}

// TeamDefensiveProfile computes the defensive profile of a team.
func TeamDefensiveProfile(team []TeamMember) DefensiveProfile {
	var profile DefensiveProfile

	// Pre-compute per-member offensive move type set.
	memberMoveSets := make([]map[string]bool, len(team))
	for i, m := range team {
		ms := make(map[string]bool, len(m.MoveTypes))
		for _, t := range m.MoveTypes {
			ms[t] = true
		}
		memberMoveSets[i] = ms
	}

	immunitySet := make(map[string]bool)
	resistanceSet := make(map[string]bool)
	weaknessMap := make(map[string]*TypeThreat)

	for _, atk := range AllTypes {
		var (
			bestResist  = 999.0 // best (lowest) multiplier any member has
			hasWeak     = false
			weakMembers []string
		)

		for _, m := range team {
			mult := memberEffectiveness(atk, m)
			if mult < bestResist {
				bestResist = mult
			}
			if mult >= 2.0 {
				hasWeak = true
				weakMembers = append(weakMembers, m.SpeciesName)
			}
		}

		if bestResist == 0.0 {
			immunitySet[atk] = true
		} else if bestResist <= 0.5 && !hasWeak {
			resistanceSet[atk] = true
		} else if hasWeak && bestResist >= 1.0 {
			// At least one weak member, no member resists.
			weaknessMap[atk] = &TypeThreat{
				Type:           atk,
				WeakMembers:    weakMembers,
				BestResistance: bestResist,
			}
		}
	}

	// Build offensive coverage to determine Uncovered.
	offCov := TeamOffensiveCoverage(team)
	uncoveredSet := make(map[string]bool, len(offCov.Gaps))
	for _, g := range offCov.Gaps {
		uncoveredSet[g] = true
	}

	// Populate profile fields.
	for atk, threat := range weaknessMap {
		_ = atk
		profile.Weaknesses = append(profile.Weaknesses, *threat)
	}
	for atk := range immunitySet {
		profile.Immunities = append(profile.Immunities, atk)
	}
	for atk := range resistanceSet {
		profile.Resistances = append(profile.Resistances, atk)
	}
	profile.Uncovered = offCov.Gaps

	sort.Slice(profile.Weaknesses, func(i, j int) bool {
		return profile.Weaknesses[i].Type < profile.Weaknesses[j].Type
	})
	sort.Strings(profile.Immunities)
	sort.Strings(profile.Resistances)

	return profile
}

// TeamOffensiveCoverage computes which defending types the team can hit super-effectively.
func TeamOffensiveCoverage(team []TeamMember) OffensiveCoverage {
	// For each defending type, collect which (member, moveType) combos hit it SE.
	coveredMap := make(map[string][]string) // def type → []"Species (MoveType)" entries

	for _, m := range team {
		for _, moveType := range m.MoveTypes {
			for _, defType := range AllTypes {
				if TypeEffectiveness(moveType, defType) >= 2.0 {
					label := capitalizeFirst(m.SpeciesName) + " (" + capitalizeFirst(moveType) + ")"
					coveredMap[defType] = append(coveredMap[defType], label)
				}
			}
		}
	}

	var cov OffensiveCoverage
	for _, defType := range AllTypes {
		entries, ok := coveredMap[defType]
		if !ok {
			cov.Gaps = append(cov.Gaps, defType)
		} else {
			// De-duplicate entries while preserving order.
			seen := make(map[string]bool)
			var deduped []string
			for _, e := range entries {
				if !seen[e] {
					seen[e] = true
					deduped = append(deduped, e)
				}
			}
			cov.Covered = append(cov.Covered, CoverageEntry{
				DefendingType: defType,
				CoveredBy:     deduped,
			})
		}
	}

	return cov
}

// CounterChain performs multi-hop type counter analysis.
// Starting from threatType, it finds types that are super effective against it
// (depth 1), then types that counter those counters (depth 2), up to maxDepth.
// maxDepth is clamped to [1, 3].
func CounterChain(threatType string, team []TeamMember, maxDepth int) []CounterHop {
	if maxDepth < 1 {
		maxDepth = 1
	}
	if maxDepth > 3 {
		maxDepth = 3
	}

	// Build a set of move types available on the team.
	teamMoveTypes := make(map[string]bool)
	for _, m := range team {
		for _, mt := range m.MoveTypes {
			teamMoveTypes[mt] = true
		}
	}

	var hops []CounterHop
	currentTargets := []string{threatType}

	for depth := 1; depth <= maxDepth; depth++ {
		// Collect all types that hit any current target super-effectively.
		counterTypeSet := make(map[string]bool)
		for _, target := range currentTargets {
			for _, atk := range AllTypes {
				if TypeEffectiveness(atk, target) >= 2.0 {
					counterTypeSet[atk] = true
				}
			}
		}

		var counterTypes []string
		for t := range counterTypeSet {
			counterTypes = append(counterTypes, t)
		}
		sort.Strings(counterTypes)

		// Check if team already covers at least one of these types.
		teamCovers := false
		for _, ct := range counterTypes {
			if teamMoveTypes[ct] {
				teamCovers = true
				break
			}
		}

		hops = append(hops, CounterHop{
			Depth:        depth,
			CounterTypes: counterTypes,
			TeamCovers:   teamCovers,
		})

		// Next iteration targets the counter types.
		currentTargets = counterTypes
		if len(currentTargets) == 0 {
			break
		}
	}

	return hops
}

// memberEffectiveness returns the damage multiplier for an attack type
// against a team member's type(s).
func memberEffectiveness(atkType string, m TeamMember) float64 {
	if m.Type2 == "" {
		return TypeEffectiveness(atkType, m.Type1)
	}
	return DualTypeEffectiveness(atkType, m.Type1, m.Type2)
}
