-- 009_pokemon_types_stats.sql
-- Adds Pokémon type, base-stat, and ability tables seeded lazily via PokeAPI.

-- Pokémon type assignments (one or two rows per form)
CREATE TABLE IF NOT EXISTS pokemon_type (
    form_id   INTEGER NOT NULL REFERENCES pokemon_form(id),
    slot      INTEGER NOT NULL,  -- 1 = primary, 2 = secondary
    type_name TEXT NOT NULL,
    PRIMARY KEY (form_id, slot)
);

-- Base stats per form
CREATE TABLE IF NOT EXISTS pokemon_stats (
    form_id    INTEGER PRIMARY KEY REFERENCES pokemon_form(id),
    hp         INTEGER NOT NULL,
    attack     INTEGER NOT NULL,
    defense    INTEGER NOT NULL,
    sp_attack  INTEGER NOT NULL,
    sp_defense INTEGER NOT NULL,
    speed      INTEGER NOT NULL
);

-- Abilities per form (slot 1/2 = regular; slot 3 = hidden — not available Gen 3)
CREATE TABLE IF NOT EXISTS pokemon_ability (
    form_id      INTEGER NOT NULL REFERENCES pokemon_form(id),
    slot         INTEGER NOT NULL,
    ability_name TEXT NOT NULL,
    PRIMARY KEY (form_id, slot)
);
