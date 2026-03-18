-- 019_move_tooltip_fields.sql
-- Add damage_class and effect_entry to the move table so the team-slot
-- move selector can show a hover tooltip with category, power, accuracy,
-- and a short description.  Both columns are nullable; rows seeded before
-- this migration will have NULL until the Pokémon is re-fetched (or never,
-- since the tooltip just omits missing fields gracefully).

ALTER TABLE move ADD COLUMN damage_class TEXT;
ALTER TABLE move ADD COLUMN effect_entry TEXT;
