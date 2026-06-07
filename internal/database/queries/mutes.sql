-- name: MuteUser :exec
INSERT INTO muted_users (user_id, reason, muted_by, expires_at, created_at)
VALUES (@user_id, @reason, @muted_by, @expires_at, NOW())
ON CONFLICT (user_id) DO UPDATE SET
    reason = EXCLUDED.reason,
    muted_by = EXCLUDED.muted_by,
    expires_at = EXCLUDED.expires_at,
    created_at = NOW();

-- name: UnmuteUser :exec
DELETE FROM muted_users
WHERE user_id = @user_id;

-- name: GetActiveMute :one
SELECT user_id, reason, muted_by, expires_at, created_at
FROM muted_users
WHERE user_id = @user_id
  AND expires_at > NOW()
LIMIT 1;

-- name: CountActiveMutedUsers :one
SELECT COUNT(*)::BIGINT
FROM muted_users
WHERE expires_at > NOW();

-- name: ListActiveMutedUsers :many
SELECT
    m.user_id,
    m.reason,
    m.muted_by,
    m.expires_at,
    m.created_at,
    c.username,
    c.first_name,
    c.last_name
FROM muted_users m
LEFT JOIN chat c ON c.chat_id = m.user_id AND c.type = 'private'
WHERE m.expires_at > NOW()
ORDER BY m.expires_at ASC
LIMIT @limit_count;
