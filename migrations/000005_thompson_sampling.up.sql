-- Thompson sampling: игрок закрепляется за вариантом, выбранным бандитом при первом заходе.
CREATE TABLE IF NOT EXISTS experiment_assignments (
    experiment_id UUID NOT NULL REFERENCES experiments(id) ON DELETE CASCADE,
    player_id     TEXT NOT NULL,
    game_id       UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (experiment_id, player_id)
);

-- Кто выбрал победителя: автор вручную или бандит по вероятности быть лучшим.
ALTER TABLE experiments
    ADD COLUMN IF NOT EXISTS winner_source TEXT NOT NULL DEFAULT ''
        CHECK (winner_source IN ('', 'manual', 'auto'));
