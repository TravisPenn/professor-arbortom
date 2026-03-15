-- DC-001: Add acquisition tracking columns to run_pokemon.

ALTER TABLE run_pokemon ADD COLUMN acquisition_type TEXT NOT NULL DEFAULT 'manual'
    CHECK(acquisition_type IN ('starter','wild','gift','trade','manual'));

ALTER TABLE run_pokemon ADD COLUMN caught_level INTEGER;

-- Backfill: rows with a met_location_id were logged via Routes → wild catches.
UPDATE run_pokemon
SET acquisition_type = 'wild',
    caught_level     = level
WHERE met_location_id IS NOT NULL
  AND acquisition_type = 'manual';

-- Backfill: rows at level 5, in party, no location → starters.
UPDATE run_pokemon
SET acquisition_type = 'starter',
    caught_level     = level
WHERE met_location_id IS NULL
  AND in_party = 1
  AND level = 5
  AND acquisition_type = 'manual';
