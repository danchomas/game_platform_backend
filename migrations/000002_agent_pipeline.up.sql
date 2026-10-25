ALTER TABLE games
    ADD COLUMN IF NOT EXISTS status        TEXT NOT NULL DEFAULT 'ready',
    ADD COLUMN IF NOT EXISTS status_detail TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS spec          JSONB;

CREATE TABLE IF NOT EXISTS generations (
    id                BIGSERIAL PRIMARY KEY,
    game_id           UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    kind              TEXT NOT NULL CHECK (kind IN ('plan','revise','build','edit')),
    status            TEXT NOT NULL DEFAULT 'running'
                      CHECK (status IN ('running','ready','ready_with_issues','failed')),
    version           INT,
    prompt_tokens     INT NOT NULL DEFAULT 0,
    completion_tokens INT NOT NULL DEFAULT 0,
    cost_usd          DOUBLE PRECISION NOT NULL DEFAULT 0,
    duration_ms       BIGINT NOT NULL DEFAULT 0,
    fix_rounds        INT NOT NULL DEFAULT 0,
    issues            JSONB NOT NULL DEFAULT '[]',
    test_report       JSONB,
    screenshot        BYTEA,
    error             TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at       TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS generations_game_idx ON generations(game_id, id DESC);
CREATE INDEX IF NOT EXISTS generations_created_idx ON generations(created_at DESC);

CREATE TABLE IF NOT EXISTS llm_calls (
    id                BIGSERIAL PRIMARY KEY,
    generation_id     BIGINT NOT NULL REFERENCES generations(id) ON DELETE CASCADE,
    role              TEXT NOT NULL,
    model             TEXT NOT NULL,
    prompt_tokens     INT NOT NULL DEFAULT 0,
    completion_tokens INT NOT NULL DEFAULT 0,
    cost_usd          DOUBLE PRECISION NOT NULL DEFAULT 0,
    latency_ms        BIGINT NOT NULL DEFAULT 0,
    finish_reason     TEXT NOT NULL DEFAULT '',
    error             TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS llm_calls_generation_idx ON llm_calls(generation_id);
