-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS download_events (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL REFERENCES chat(chat_id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    chat_type chat_type NOT NULL,
    extractor_id VARCHAR(30) NOT NULL,
    content_id VARCHAR(150) NOT NULL,
    content_url TEXT NOT NULL,
    item_count INT NOT NULL,
    total_file_size BIGINT NOT NULL DEFAULT 0,
    from_cache BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_download_events_chat_created_at
    ON download_events (chat_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_download_events_user_created_at
    ON download_events (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_download_events_extractor_created_at
    ON download_events (extractor_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS download_events;
-- +goose StatementEnd
