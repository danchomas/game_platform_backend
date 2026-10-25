DROP INDEX IF EXISTS games_experiment_idx;
ALTER TABLE games
    DROP COLUMN IF EXISTS variant_direction,
    DROP COLUMN IF EXISTS variant_label,
    DROP COLUMN IF EXISTS experiment_id;
DROP TABLE IF EXISTS experiments;
