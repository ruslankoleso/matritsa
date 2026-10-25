-- name: EnsureChatThread :one
-- The client's single thread, created on first use.
INSERT INTO chat_threads (user_id) VALUES ($1)
ON CONFLICT (user_id) DO UPDATE SET user_id = EXCLUDED.user_id
RETURNING *;

-- name: GetChatThread :one
SELECT * FROM chat_threads WHERE id = $1;

-- name: InsertChatMessage :one
INSERT INTO chat_messages (thread_id, author, text, source, off_topic, agent_name)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListChatMessages :many
-- The latest messages of a thread, oldest first.
SELECT * FROM (
    SELECT * FROM chat_messages WHERE thread_id = $1 ORDER BY id DESC LIMIT $2
) recent ORDER BY id;

-- name: RequestChatOperator :one
-- bot -> waiting; no row means the client was already waiting or talking to a person.
UPDATE chat_threads SET status = 'waiting', requested_at = now(), updated_at = now()
WHERE id = $1 AND status = 'bot'
RETURNING *;

-- name: JoinChatThread :one
-- A specialist takes a waiting thread, or rejoins their own; no row means somebody else has it.
UPDATE chat_threads SET status = 'live', agent_id = $2, updated_at = now()
WHERE id = $1 AND ((status = 'waiting') OR (status = 'live' AND agent_id = $2))
RETURNING *;

-- name: ReleaseChatThread :one
-- Back to the helper: the specialist is done or the client cancelled. No row means it was already with the helper.
UPDATE chat_threads SET status = 'bot', agent_id = NULL, requested_at = NULL, updated_at = now()
WHERE id = $1 AND status <> 'bot'
RETURNING *;

-- name: TouchChatThread :exec
UPDATE chat_threads SET updated_at = now() WHERE id = $1;

-- name: ListSupportChats :many
-- Threads that need a person or have one: waiting ones first, the longest waiting at the top.
SELECT t.id, t.status, t.requested_at, t.updated_at, t.agent_id,
       u.full_name AS client_name, COALESCE(c.name, '') AS company_name,
       COALESCE(a.full_name, '') AS agent_name,
       COALESCE((SELECT m.text FROM chat_messages m WHERE m.thread_id = t.id AND m.author IN ('client', 'agent') ORDER BY m.id DESC LIMIT 1), '')::text AS last_text
FROM chat_threads t
JOIN users u ON u.id = t.user_id
LEFT JOIN companies c ON c.id = u.company_id
LEFT JOIN users a ON a.id = t.agent_id
WHERE t.status <> 'bot'
ORDER BY (t.status = 'waiting') DESC, t.requested_at;

-- name: GetChatClient :one
SELECT u.id AS user_id, u.full_name, COALESCE(c.name, '') AS company_name, COALESCE(c.inn, '') AS inn
FROM chat_threads t
JOIN users u ON u.id = t.user_id
LEFT JOIN companies c ON c.id = u.company_id
WHERE t.id = $1;
