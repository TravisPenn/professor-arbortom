-- 013_merge_pokemon_tables.sql
-- SC-001 (phase 1): introduce consolidated pokemon table and backfill data.
--
-- NOTE: This pass is intentionally non-destructive. Legacy tables remain so
-- existing handlers/legality queries keep working while code is migrated.

CREATE TABLE IF NOT EXISTS pokemon (
    id           INTEGER PRIMARY KEY,   -- compatible with existing pokemon_form.id
    species_name TEXT NOT NULL,
    form_name    TEXT NOT NULL,
    type1        TEXT NOT NULL,
    type2        TEXT,
    hp           INTEGER NOT NULL DEFAULT 0,
    attack       INTEGER NOT NULL DEFAULT 0,
    defense      INTEGER NOT NULL DEFAULT 0,
    sp_attack    INTEGER NOT NULL DEFAULT 0,
    sp_defense   INTEGER NOT NULL DEFAULT 0,
    speed        INTEGER NOT NULL DEFAULT 0,
    ability1     TEXT,
    ability2     TEXT
);

INSERT OR REPLACE INTO pokemon (
    id, species_name, form_name, type1, type2,
    hp, attack, defense, sp_attack, sp_defense, speed, ability1, ability2
)
SELECT
    pf.id,
    ps.name,
    pf.form_name,
    COALESCE((SELECT pt.type_name FROM pokemon_type pt WHERE pt.form_id = pf.id AND pt.slot = 1), 'normal'),
    (SELECT pt.type_name FROM pokemon_type pt WHERE pt.form_id = pf.id AND pt.slot = 2),
    COALESCE(st.hp, 0),
    COALESCE(st.attack, 0),
    COALESCE(st.defense, 0),
    COALESCE(st.sp_attack, 0),
    COALESCE(st.sp_defense, 0),
    COALESCE(st.speed, 0),
    (SELECT pa.ability_name FROM pokemon_ability pa WHERE pa.form_id = pf.id AND pa.slot = 1),
    (SELECT pa.ability_name FROM pokemon_ability pa WHERE pa.form_id = pf.id AND pa.slot = 2)
FROM pokemon_form pf
JOIN pokemon_species ps ON ps.id = pf.species_id
LEFT JOIN pokemon_stats st ON st.form_id = pf.id;

CREATE INDEX IF NOT EXISTS idx_pokemon_species_name ON pokemon(species_name);
