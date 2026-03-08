package legality

import "testing"

// ── toFloat64 ────────────────────────────────────────────────────────────────

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want float64
	}{
		{"float64", float64(3.14), 3.14},
		{"int", int(5), 5.0},
		{"int64", int64(7), 7.0},
		{"string", "hello", 0},
		{"nil", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toFloat64(tt.in)
			if got != tt.want {
				t.Errorf("toFloat64(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// ── isEvoCurrentlyPossible ───────────────────────────────────────────────────

func rs0() *RunState {
	return &RunState{
		ActiveRules: map[string]bool{},
		RuleParams:  map[string]map[string]interface{}{},
		Flags:       map[string]bool{},
	}
}

func TestIsEvoCurrentlyPossible_LevelUp(t *testing.T) {
	t.Run("no conditions always possible", func(t *testing.T) {
		rs := rs0()
		evo := Evolution{Trigger: "level-up", Conditions: map[string]interface{}{}}
		if !isEvoCurrentlyPossible(rs, evo, 0) {
			t.Fatal("expected possible")
		}
	})

	t.Run("min_level within cap", func(t *testing.T) {
		rs := rs0()
		evo := Evolution{Trigger: "level-up", Conditions: map[string]interface{}{"min_level": float64(30)}}
		if !isEvoCurrentlyPossible(rs, evo, 50) {
			t.Fatal("expected possible when min_level <= cap")
		}
	})

	t.Run("min_level equals cap is possible", func(t *testing.T) {
		rs := rs0()
		evo := Evolution{Trigger: "level-up", Conditions: map[string]interface{}{"min_level": float64(30)}}
		if !isEvoCurrentlyPossible(rs, evo, 30) {
			t.Fatal("expected possible when min_level == cap")
		}
	})

	t.Run("min_level exceeds cap", func(t *testing.T) {
		rs := rs0()
		evo := Evolution{Trigger: "level-up", Conditions: map[string]interface{}{"min_level": float64(50)}}
		if isEvoCurrentlyPossible(rs, evo, 30) {
			t.Fatal("expected blocked when min_level > cap")
		}
	})

	t.Run("min_level with no cap (cap=0) always possible", func(t *testing.T) {
		rs := rs0()
		evo := Evolution{Trigger: "level-up", Conditions: map[string]interface{}{"min_level": float64(100)}}
		if !isEvoCurrentlyPossible(rs, evo, 0) {
			t.Fatal("expected possible with no cap")
		}
	})

	t.Run("friendship flag set", func(t *testing.T) {
		rs := rs0()
		rs.Flags["story.high_friendship"] = true
		evo := Evolution{Trigger: "level-up", Conditions: map[string]interface{}{"friendship": true}}
		if !isEvoCurrentlyPossible(rs, evo, 0) {
			t.Fatal("expected possible with friendship flag")
		}
	})

	t.Run("friendship flag not set", func(t *testing.T) {
		rs := rs0()
		evo := Evolution{Trigger: "level-up", Conditions: map[string]interface{}{"friendship": true}}
		if isEvoCurrentlyPossible(rs, evo, 0) {
			t.Fatal("expected not possible without friendship flag")
		}
	})
}

func TestIsEvoCurrentlyPossible_UseItem(t *testing.T) {
	rs := rs0()
	evo := Evolution{Trigger: "use-item", Conditions: map[string]interface{}{}}
	if !isEvoCurrentlyPossible(rs, evo, 0) {
		t.Fatal("expected use-item to always be possible")
	}
}

func TestIsEvoCurrentlyPossible_Trade(t *testing.T) {
	t.Run("trade allowed", func(t *testing.T) {
		rs := rs0()
		rs.ActiveRules["no_trade_evolutions"] = false
		evo := Evolution{Trigger: "trade", Conditions: map[string]interface{}{}}
		if !isEvoCurrentlyPossible(rs, evo, 0) {
			t.Fatal("expected trade possible when rule disabled")
		}
	})

	t.Run("trade blocked by rule", func(t *testing.T) {
		rs := rs0()
		rs.ActiveRules["no_trade_evolutions"] = true
		evo := Evolution{Trigger: "trade", Conditions: map[string]interface{}{}}
		if isEvoCurrentlyPossible(rs, evo, 0) {
			t.Fatal("expected trade blocked when no_trade_evolutions is true")
		}
	})
}

func TestIsEvoCurrentlyPossible_UnknownTrigger(t *testing.T) {
	rs := rs0()
	evo := Evolution{Trigger: "unknown-trigger", Conditions: map[string]interface{}{}}
	if !isEvoCurrentlyPossible(rs, evo, 0) {
		t.Fatal("expected unknown triggers to default to possible")
	}
}
