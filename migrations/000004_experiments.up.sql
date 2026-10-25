-- Мультигенерация: эксперимент объединяет несколько игр-вариантов по одному запросу
-- и раздаёт их игрокам по общей ссылке /e/{id}.
CREATE TABLE IF NOT EXISTS experiments (
    id          UUID PRIMARY KEY,
    owner_id    UUID NOT NULL REFERENCES users(id),
    title       TEXT NOT NULL DEFAULT '',
    prompt      TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','running','finished')),
    winner_id   UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    launched_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS experiments_owner_idx ON experiments(owner_id, created_at DESC);

ALTER TABLE games
    ADD COLUMN IF NOT EXISTS experiment_id     UUID REFERENCES experiments(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS variant_label     TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS variant_direction TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS games_experiment_idx ON games(experiment_id) WHERE experiment_id IS NOT NULL;

ALTER TABLE experiments
    ADD CONSTRAINT experiments_winner_fk FOREIGN KEY (winner_id) REFERENCES games(id) ON DELETE SET NULL;
