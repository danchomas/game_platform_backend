CREATE TABLE IF NOT EXISTS users (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'player' CHECK (role IN ('manager','player')),
    access_key  TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS games (
    id          UUID PRIMARY KEY,
    owner_id    UUID NOT NULL REFERENCES users(id),
    title       TEXT NOT NULL DEFAULT 'Без названия',
    genre       TEXT NOT NULL DEFAULT '',
    audience    TEXT NOT NULL DEFAULT '',
    prompt      TEXT NOT NULL,
    html        TEXT NOT NULL DEFAULT '',
    version     INT  NOT NULL DEFAULT 0,
    published   BOOLEAN NOT NULL DEFAULT false,
    plays       BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS games_owner_idx ON games(owner_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS games_pub_idx   ON games(published, plays DESC);
CREATE INDEX IF NOT EXISTS games_pub_new   ON games(published, created_at DESC);

CREATE TABLE IF NOT EXISTS game_versions (
    game_id     UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    version     INT  NOT NULL,
    html        TEXT NOT NULL,
    instruction TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (game_id, version)
);

CREATE TABLE IF NOT EXISTS messages (
    id          BIGSERIAL PRIMARY KEY,
    game_id     UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    role        TEXT NOT NULL CHECK (role IN ('user','assistant')),
    content     TEXT NOT NULL,
    version     INT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS messages_game_idx ON messages(game_id, id);

CREATE TABLE IF NOT EXISTS events (
    id          BIGSERIAL PRIMARY KEY,
    game_id     UUID NOT NULL,
    session_id  TEXT NOT NULL,
    player_id   TEXT NOT NULL DEFAULT '',
    type        TEXT NOT NULL,
    result      TEXT NOT NULL DEFAULT '',
    score       DOUBLE PRECISION,
    duration_ms BIGINT,
    payload     JSONB,
    ts          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS events_game_ts_idx ON events(game_id, ts DESC);
CREATE INDEX IF NOT EXISTS events_game_type_idx ON events(game_id, type);
