-- +goose Up
-- +goose StatementBegin
ALTER TABLE chat
    ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS username TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS first_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_chat_type_last_seen_at
    ON chat (type, last_seen_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_chat_type_last_seen_at;

ALTER TABLE chat
    DROP COLUMN IF EXISTS last_seen_at,
    DROP COLUMN IF EXISTS last_name,
    DROP COLUMN IF EXISTS first_name,
    DROP COLUMN IF EXISTS username,
    DROP COLUMN IF EXISTS title;
-- +goose StatementEnd
