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
    c.username,
    c.first_name,
    c.last_name
FROM banned_users b
LEFT JOIN chat c ON c.chat_id = b.user_id AND c.type = 'private'
ORDER BY b.created_at DESC
LIMIT @limit_count;
