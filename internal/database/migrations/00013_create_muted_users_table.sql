-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS muted_users (
    user_id BIGINT PRIMARY KEY,
    reason TEXT NOT NULL DEFAULT '',
    muted_by BIGINT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_muted_users_expires_at
    ON muted_users (expires_at);

CREATE INDEX IF NOT EXISTS idx_muted_users_created_at
    ON muted_users (created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_muted_users_created_at;
DROP INDEX IF EXISTS idx_muted_users_expires_at;
DROP TABLE IF EXISTS muted_users;
-- +goose StatementEnd
