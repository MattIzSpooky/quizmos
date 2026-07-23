-- The app only offers a small, curated set of color IDs (see
-- service.PlayerColorIDs) — enforced in the application layer, not a DB
-- check constraint, so the palette can change without a migration.
ALTER TABLE players ADD COLUMN color text NOT NULL DEFAULT 'nebula';
