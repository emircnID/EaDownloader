-- name: CreateDownloadEvent :exec
INSERT INTO download_events (
    chat_id,
    user_id,
    chat_type,
    extractor_id,
    content_id,
    content_url,
    item_count,
    total_file_size,
    from_cache
) VALUES (
    @chat_id,
    @user_id,
    @chat_type,
    @extractor_id,
    @content_id,
    @content_url,
    @item_count,
    @total_file_size,
    @from_cache
);

-- name: GetChatDownloadSummary :one
SELECT
    COUNT(*)::BIGINT AS downloads,
    COALESCE(SUM(item_count), 0)::BIGINT AS items,
    COALESCE(SUM(total_file_size), 0)::BIGINT AS total_size,
    MAX(created_at)::TIMESTAMP WITH TIME ZONE AS last_download_at
FROM download_events
WHERE chat_id = @chat_id;

-- name: GetUserDownloadSummary :one
SELECT
    COUNT(*)::BIGINT AS downloads,
    COALESCE(SUM(item_count), 0)::BIGINT AS items,
    COALESCE(SUM(total_file_size), 0)::BIGINT AS total_size,
    MAX(created_at)::TIMESTAMP WITH TIME ZONE AS last_download_at
FROM download_events
WHERE user_id = @user_id;

-- name: ListChatPlatformStats :many
SELECT
    extractor_id,
    COUNT(*)::BIGINT AS downloads,
    COALESCE(SUM(total_file_size), 0)::BIGINT AS total_size
FROM download_events
WHERE chat_id = @chat_id
GROUP BY extractor_id
ORDER BY downloads DESC, total_size DESC
LIMIT @limit_count;

-- name: ListUserPlatformStats :many
SELECT
    extractor_id,
    COUNT(*)::BIGINT AS downloads,
    COALESCE(SUM(total_file_size), 0)::BIGINT AS total_size
FROM download_events
WHERE user_id = @user_id
GROUP BY extractor_id
ORDER BY downloads DESC, total_size DESC
LIMIT @limit_count;

-- name: ListChatRecentDownloadEvents :many
SELECT
    de.extractor_id,
    de.content_id,
    de.content_url,
    de.user_id,
    uc.username AS user_username,
    uc.first_name AS user_first_name,
    uc.last_name AS user_last_name,
    de.item_count,
    de.total_file_size,
    de.from_cache,
    de.created_at
FROM download_events de
LEFT JOIN chat uc ON uc.chat_id = de.user_id
WHERE de.chat_id = @chat_id
ORDER BY de.created_at DESC
LIMIT @limit_count;

-- name: ListUserRecentDownloadEvents :many
SELECT
    de.extractor_id,
    de.content_id,
    de.content_url,
    de.chat_id,
    de.chat_type,
    c.title AS chat_title,
    c.username AS chat_username,
    de.item_count,
    de.total_file_size,
    de.from_cache,
    de.created_at
FROM download_events de
LEFT JOIN chat c ON c.chat_id = de.chat_id
WHERE de.user_id = @user_id
ORDER BY de.created_at DESC
LIMIT @limit_count;
