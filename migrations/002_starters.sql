-- PokemonProfessor migration 002 — starter Pokémon per game version.

CREATE TABLE IF NOT EXISTS game_starter (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    version_id INTEGER NOT NULL REFERENCES game_version(id),
    form_id    INTEGER NOT NULL REFERENCES pokemon_form(id),
    priority   INTEGER NOT NULL DEFAULT 0,
    UNIQUE(version_id, form_id)
);

-- Pre-seed species so the starter picker works before PokeAPI seeding completes.
-- INSERT OR IGNORE ensures PokeAPI-fetched rows with identical data are not duplicated.
INSERT OR IGNORE INTO pokemon_species (id, name) VALUES
  (1,   'bulbasaur'),
  (4,   'charmander'),
  (7,   'squirtle'),
  (252, 'treecko'),
  (255, 'torchic'),
  (258, 'mudkip');

INSERT OR IGNORE INTO pokemon_form (id, species_id, form_name) VALUES
  (1,   1,   'default'),
  (4,   4,   'default'),
  (7,   7,   'default'),
  (252, 252, 'default'),
  (255, 255, 'default'),
  (258, 258, 'default');

-- FireRed / LeafGreen — Kanto starters: Bulbasaur, Charmander, Squirtle
INSERT OR IGNORE INTO game_starter (version_id, form_id, priority) VALUES
  (10, 1, 0), (10, 4, 1), (10, 7, 2),
  (11, 1, 0), (11, 4, 1), (11, 7, 2);

-- Ruby / Sapphire / Emerald — Hoenn starters: Treecko, Torchic, Mudkip
INSERT OR IGNORE INTO game_starter (version_id, form_id, priority) VALUES
  (6,  252, 0), (6,  255, 1), (6,  258, 2),
  (7,  252, 0), (7,  255, 1), (7,  258, 2),
  (8,  252, 0), (8,  255, 1), (8,  258, 2);
