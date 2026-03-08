package legality

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// LoadEvolutionGraph loads all evolution_condition rows from the DB into an
// in-memory adjacency list. Safe to call at Coach query time; the table is
// small enough that full scans are fast.
func LoadEvolutionGraph(db *sql.DB) (*EvolutionGraph, error) {
	rows, err := db.Query(`
		SELECT ec.from_form_id, ec.to_form_id,
		       ps.name AS to_species_name,
		       ec.trigger, ec.conditions_json
		FROM evolution_condition ec
		JOIN pokemon_form pf ON pf.id = ec.to_form_id
		JOIN pokemon_species ps ON ps.id = pf.species_id
	`)
	if err != nil {
		return nil, fmt.Errorf("pathfind: load evolution graph: %w", err)
	}
	defer rows.Close()

	g := &EvolutionGraph{edges: make(map[int][]EvolutionEdge)}
	for rows.Next() {
		var fromID int
		var edge EvolutionEdge
		var condJSON string
		if err := rows.Scan(&fromID, &edge.ToFormID, &edge.ToSpeciesName, &edge.Trigger, &condJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(condJSON), &edge.Conditions); err != nil {
			edge.Conditions = map[string]interface{}{}
		}
		g.edges[fromID] = append(g.edges[fromID], edge)
	}
	return g, rows.Err()
}

// FindEvolutionPaths performs BFS from fromFormID and returns all reachable
// evolution targets, annotated with feasibility given the run state. The BFS
// depth is bounded by the maximum chain length (Caterpie→Metapod→Butterfree =
// 2 hops) so there is no risk of infinite loops even with malformed data.
func FindEvolutionPaths(graph *EvolutionGraph, fromFormID int, rs *RunState, levelCap int) []EvolutionPath {
	type queueItem struct {
		formID int
		path   []EvolutionStep
	}

	var results []EvolutionPath
	visited := make(map[int]bool)
	visited[fromFormID] = true

	queue := []queueItem{{formID: fromFormID}}
	const maxDepth = 5 // safety: no real evolution chain exceeds 3 hops

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if len(cur.path) >= maxDepth {
			continue
		}

		edges, ok := graph.edges[cur.formID]
		if !ok {
			continue
		}

		for _, edge := range edges {
			if visited[edge.ToFormID] {
				continue
			}

			step := EvolutionStep{
				FromFormID:    cur.formID,
				ToFormID:      edge.ToFormID,
				ToSpeciesName: edge.ToSpeciesName,
				Trigger:       edge.Trigger,
				Conditions:    edge.Conditions,
			}
			step.Possible, step.BlockedBy = evalStepFeasibility(rs, edge, levelCap)

			steps := make([]EvolutionStep, len(cur.path)+1)
			copy(steps, cur.path)
			steps[len(cur.path)] = step

			// Determine overall path legality.
			fullyLegal := true
			blockReason := ""
			for _, s := range steps {
				if !s.Possible {
					fullyLegal = false
					if s.BlockedBy != nil {
						blockReason = *s.BlockedBy
					} else {
						blockReason = "unknown"
					}
					break
				}
			}

			results = append(results, EvolutionPath{
				Steps:       steps,
				FullyLegal:  fullyLegal,
				BlockReason: blockReason,
			})

			// Explore further evolutions from this target.
			visited[edge.ToFormID] = true
			queue = append(queue, queueItem{formID: edge.ToFormID, path: steps})
		}
	}

	return results
}

// evalStepFeasibility checks whether a single evolution edge is currently
// achievable given the run state. Returns (possible, blockedByReason).
func evalStepFeasibility(rs *RunState, edge EvolutionEdge, levelCap int) (bool, *string) {
	cond := edge.Conditions

	switch edge.Trigger {
	case "level-up":
		if minLevel, ok := cond["min_level"]; ok {
			minLvl := int(toFloat64(minLevel))
			if levelCap > 0 && minLvl > levelCap {
				r := "level_cap"
				return false, &r
			}
		}
		if _, ok := cond["friendship"]; ok {
			if !rs.Flags["story.high_friendship"] {
				r := "missing: high friendship"
				return false, &r
			}
		}
		if heldItemID, ok := cond["held_item_id"]; ok {
			_ = heldItemID // would need run_item check; conservatively allow
		}
		return true, nil

	case "use-item":
		// Item availability checked separately; mark as possible here.
		return true, nil

	case "trade":
		if rs.ActiveRules["no_trade_evolutions"] {
			r := "no_trade_evolutions"
			return false, &r
		}
		return true, nil

	default:
		return true, nil
	}
}

// MoveDelayAnalysis identifies level-up moves the pre-evolution learns at a
// lower level than its evolutions — suggesting the player may want to delay.
func MoveDelayAnalysis(db *sql.DB, fromFormID int, path EvolutionPath, versionGroupID int) ([]MoveDelayNote, error) {
	if len(path.Steps) == 0 {
		return nil, nil
	}

	// Get direct evolution's form ID (first step).
	evoFormID := path.Steps[0].ToFormID

	// Pre-evolution level-up moves.
	preRows, err := db.Query(`
		SELECT m.name, le.level_learned
		FROM learnset_entry le
		JOIN move m ON m.id = le.move_id
		WHERE le.form_id = ? AND le.version_group_id = ? AND le.learn_method = 'level-up'
		ORDER BY le.level_learned
	`, fromFormID, versionGroupID)
	if err != nil {
		return nil, fmt.Errorf("pathfind: pre-evo learnset: %w", err)
	}
	defer preRows.Close()

	type levelEntry struct {
		name  string
		level int
	}
	var preMoves []levelEntry
	for preRows.Next() {
		var e levelEntry
		if err := preRows.Scan(&e.name, &e.level); err != nil {
			return nil, err
		}
		preMoves = append(preMoves, e)
	}
	if len(preMoves) == 0 {
		return nil, nil
	}

	// Evolution's level-up moves.
	evoRows, err := db.Query(`
		SELECT m.name, MIN(le.level_learned)
		FROM learnset_entry le
		JOIN move m ON m.id = le.move_id
		WHERE le.form_id = ? AND le.version_group_id = ? AND le.learn_method = 'level-up'
		GROUP BY m.name
	`, evoFormID, versionGroupID)
	if err != nil {
		return nil, fmt.Errorf("pathfind: evo learnset: %w", err)
	}
	defer evoRows.Close()

	evoLearnset := make(map[string]int)
	for evoRows.Next() {
		var name string
		var lvl int
		if err := evoRows.Scan(&name, &lvl); err != nil {
			return nil, err
		}
		evoLearnset[name] = lvl
	}

	var notes []MoveDelayNote
	for _, pre := range preMoves {
		evoLvl, ok := evoLearnset[pre.name]

		var note MoveDelayNote
		note.MoveName = pre.name
		note.PreEvoLevel = pre.level

		if !ok {
			// Evolution never learns this move via level-up.
			note.PostEvoLevel = 0
			note.Recommendation = "delay"
		} else if pre.level < evoLvl {
			note.PostEvoLevel = evoLvl
			note.Recommendation = "delay"
		} else if evoLvl < pre.level {
			note.PostEvoLevel = evoLvl
			note.Recommendation = "evolve_now"
		} else {
			note.PostEvoLevel = evoLvl
			note.Recommendation = "neutral"
		}

		if note.Recommendation != "neutral" {
			notes = append(notes, note)
		}
	}

	return notes, nil
}
