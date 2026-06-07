-- name: GetOrCreateChat :one
WITH upsert_chat AS (
    INSERT INTO chat (chat_id, type, title, username, first_name, last_name, last_seen_at)
    VALUES (@chat_id, @type, @title, @username, @first_name, @last_name, NOW())
    ON CONFLICT (chat_id) DO UPDATE SET
        type = EXCLUDED.type,
        title = EXCLUDED.title,
        username = EXCLUDED.username,
        first_name = EXCLUDED.first_name,
        last_name = EXCLUDED.last_name,
        last_seen_at = NOW(),
        updated_at = NOW()
    RETURNING *
),
upsert_settings AS (
    INSERT INTO settings (chat_id, language, captions, silent, nsfw, media_album_limit, delete_links)
    VALUES (@chat_id, @language, @captions, @silent, @nsfw, @media_album_limit, @delete_links)
    ON CONFLICT (chat_id) DO UPDATE SET
        language = CASE 
            WHEN settings.language = 'XX' THEN EXCLUDED.language 
            ELSE settings.language 
        END
    RETURNING *
),
final_chat AS (
    SELECT * FROM upsert_chat
    UNION ALL
    SELECT * FROM chat WHERE chat_id = @chat_id AND NOT EXISTS (SELECT 1 FROM upsert_chat)
),
final_settings AS (
    SELECT * FROM upsert_settings
)
SELECT 
    c.chat_id,
    c.type,
    c.title,
    c.username,
    c.first_name,
    c.last_name,
    c.last_seen_at,
    s.nsfw,
    s.media_album_limit,
    s.captions,
    s.silent,
    s.language,
    s.disabled_extractors,
    s.delete_links
FROM final_chat c 
JOIN final_settings s ON s.chat_id = c.chat_id;

-- name: GetChatByID :one
SELECT
    c.chat_id,
    c.type,
    c.title,
    c.username,
    c.first_name,
    c.last_name,
    s.language,
    c.created_at,
    c.last_seen_at
FROM chat c
JOIN settings s USING (chat_id)
WHERE c.chat_id = @chat_id
LIMIT 1;

-- name: GetPrivateChatByUsername :one
SELECT
    c.chat_id,
    c.type,
    c.title,
    c.username,
    c.first_name,
    c.last_name,
    s.language,
    c.created_at,
    c.last_seen_at
FROM chat c
JOIN settings s USING (chat_id)
WHERE c.type = 'private'
  AND LOWER(c.username) = LOWER(@username)
LIMIT 1;
