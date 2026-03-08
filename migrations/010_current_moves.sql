-- 010_current_moves.sql
-- Tracks the 1-4 moves currently assigned to each party Pokemon.

CREATE TABLE IF NOT EXISTS run_pokemon_move (
    run_pokemon_id INTEGER NOT NULL REFERENCES run_pokemon(id) ON DELETE CASCADE,
    slot           INTEGER NOT NULL,  -- 1-4
    move_id        INTEGER NOT NULL REFERENCES move(id),
    PRIMARY KEY (run_pokemon_id, slot)
);
