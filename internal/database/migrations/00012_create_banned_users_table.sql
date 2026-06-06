-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS banned_users (
    user_id BIGINT PRIMARY KEY,
    reason TEXT NOT NULL DEFAULT '',
    banned_by BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_banned_users_created_at
    ON banned_users (created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_banned_users_created_at;
DROP TABLE IF EXISTS banned_users;
-- +goose StatementEnd
