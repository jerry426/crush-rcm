-- name: GetMessage :one
SELECT *
FROM ai_conversation_turns
WHERE id = $1 LIMIT 1;

-- name: ListMessagesBySession :many
SELECT t.*
FROM ai_conversation_turns t
JOIN ai_conversations c ON t.id_conversation = c.id
WHERE c.session_id = $1
ORDER BY t.turn_number ASC;

-- name: CreateMessage :one
INSERT INTO ai_conversation_turns (
    id,
    id_conversation,
    turn_number,
    agent,
    role,
    content,
    parts,
    model,
    provider,
    timestamp
)
SELECT
    $1::uuid,
    c.id,
    COALESCE((SELECT MAX(turn_number) FROM ai_conversation_turns WHERE id_conversation = c.id), 0) + 1,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    CURRENT_TIMESTAMP
FROM ai_conversations c
WHERE c.session_id = $2
RETURNING *;

-- name: UpdateMessage :exec
UPDATE ai_conversation_turns
SET
    content = $1,
    parts = $2,
    finished_at = $3
WHERE id = $4;

-- name: DeleteMessage :exec
DELETE FROM ai_conversation_turns
WHERE id = $1;

-- name: DeleteSessionMessages :exec
DELETE FROM ai_conversation_turns t
USING ai_conversations c
WHERE t.id_conversation = c.id
AND c.session_id = $1;
