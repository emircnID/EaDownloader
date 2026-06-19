-- name: BanUser :one
INSERT INTO banned_users (user_id, reason, banned_by, created_at)
VALUES (@user_id, @reason, @banned_by, NOW())
ON CONFLICT (user_id) DO UPDATE SET
    reason = EXCLUDED.reason,
    banned_by = EXCLUDED.banned_by,
    created_at = NOW()
RETURNING *;

-- name: UnbanUser :exec
DELETE FROM banned_users
WHERE user_id = @user_id;

-- name: IsUserBanned :one
SELECT EXISTS (
    SELECT 1
    FROM banned_users
    WHERE user_id = @user_id
)::BOOLEAN;

-- name: CountBannedUsers :one
SELECT COUNT(*)::BIGINT
FROM banned_users;

-- name: ListBannedUsers :many
SELECT
    b.user_id,
    b.reason,
    b.banned_by,
    b.created_at,
    COALESCE(c.username, '') AS username,
    COALESCE(c.first_name, '') AS first_name,
    COALESCE(c.last_name, '') AS last_name
FROM banned_users b
LEFT JOIN chat c ON c.chat_id = b.user_id AND c.type = 'private'
WHERE b.user_id > 0
ORDER BY b.created_at DESC
LIMIT @limit_count;

-- name: CountBannedChatsByType :one
SELECT COUNT(*)::BIGINT
FROM banned_users b
LEFT JOIN chat c ON c.chat_id = b.user_id
WHERE (
    @type::chat_type = 'private'
    AND b.user_id > 0
    AND (c.type IS NULL OR c.type = 'private')
) OR c.type = @type;

-- name: ListBannedChatsByType :many
SELECT
    b.user_id,
    b.reason,
    b.banned_by,
    b.created_at,
    c.title,
    c.username,
    c.first_name,
    c.last_name
FROM banned_users b
JOIN chat c ON c.chat_id = b.user_id
WHERE c.type = @type
ORDER BY b.created_at DESC
LIMIT @limit_count;
