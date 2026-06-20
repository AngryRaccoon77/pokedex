-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS pokemon (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    power INTEGER NOT NULL DEFAULT 0,
    type TEXT NOT NULL,
    catchable BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pokemon_catchable ON pokemon(catchable);
CREATE INDEX IF NOT EXISTS idx_pokemon_created_at ON pokemon(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_pokemon_type ON pokemon(type);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS pokemon;
-- +goose StatementEnd
