-- Migration 003: merge run_party + run_box into a single run_pokemon table.
--
-- run_pokemon is the canonical record for every Pokémon the player owns.
-- in_party=1 means it is currently on the active team; party_slot holds 1-6.
-- All battle-config columns (moves_json, held_item_id) live here too.

CREATE TABLE IF NOT EXISTS run_pokemon (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id          INTEGER NOT NULL REFERENCES run(id),
    form_id         INTEGER NOT NULL REFERENCES pokemon_form(id),
    level           INTEGER NOT NULL CHECK(level BETWEEN 1 AND 100),
    met_location_id INTEGER REFERENCES location(id),
    is_alive        INTEGER NOT NULL DEFAULT 1,
    in_party        INTEGER NOT NULL DEFAULT 0,
    party_slot      INTEGER CHECK(party_slot BETWEEN 1 AND 6),
    moves_json      TEXT NOT NULL DEFAULT '[]',
    held_item_id    INTEGER REFERENCES item(id)
);

-- Enforce uniqueness of active party slots per run.
CREATE UNIQUE INDEX IF NOT EXISTS ux_run_pokemon_party_slot
    ON run_pokemon(run_id, party_slot) WHERE in_party = 1;

-- ─── data migration ───────────────────────────────────────────────────────────

-- Phase 1: migrate every run_box entry, pulling in run_party data where the
-- same form_id had a matching party slot in the same run.
INSERT INTO run_pokemon (run_id, form_id, level, met_location_id, is_alive,
                         in_party, party_slot, moves_json, held_item_id)
SELECT
    rb.run_id,
    rb.form_id,
    COALESCE(rp.level, rb.level),
    rb.met_location_id,
    rb.is_alive,
    CASE WHEN rp.slot IS NOT NULL THEN 1 ELSE 0 END,
    rp.slot,
    COALESCE(rp.moves_json, '[]'),
    rp.held_item_id
FROM run_box rb
LEFT JOIN run_party rp
    ON rp.run_id = rb.run_id
    AND rp.form_id = rb.form_id
    AND rp.rowid = (
        SELECT MIN(rowid) FROM run_party rp2
        WHERE rp2.run_id = rb.run_id AND rp2.form_id = rb.form_id
    );

-- Phase 2: any run_party entries that had no matching run_box entry.
INSERT INTO run_pokemon (run_id, form_id, level, is_alive,
                         in_party, party_slot, moves_json, held_item_id)
SELECT rp.run_id, rp.form_id, rp.level, 1, 1, rp.slot, rp.moves_json, rp.held_item_id
FROM run_party rp
WHERE NOT EXISTS (
    SELECT 1 FROM run_box rb
    WHERE rb.run_id = rp.run_id AND rb.form_id = rp.form_id
);

DROP TABLE IF EXISTS run_party;
DROP TABLE IF EXISTS run_box;
