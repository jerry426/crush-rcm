-- name: CreateSession :one
INSERT INTO ai_conversations (
    session_id,
    parent_conversation_id,
    title,
    model_provider,
    model_id,
    start_time,
    last_active,
    status
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP,
    'active'
) RETURNING id, session_id, title, task, goal, start_time, end_time, last_active, status,
    participants, total_turns, total_tokens_input, total_tokens_output, metadata, tags,
    created_at, updated_at, parent_conversation_id, cost_usd, summary_turn_id;

-- name: GetSessionByID :one
SELECT id, session_id, title, task, goal, start_time, end_time, last_active, status,
    participants, total_turns, total_tokens_input, total_tokens_output, metadata, tags,
    created_at, updated_at, parent_conversation_id, cost_usd, summary_turn_id
FROM ai_conversations
WHERE session_id = $1 LIMIT 1;

-- name: ListSessions :many
SELECT id, session_id, title, task, goal, start_time, end_time, last_active, status,
    participants, total_turns, total_tokens_input, total_tokens_output, metadata, tags,
    created_at, updated_at, parent_conversation_id, cost_usd, summary_turn_id
FROM ai_conversations
WHERE parent_conversation_id is NULL
ORDER BY created_at DESC;

-- name: UpdateSession :one
UPDATE ai_conversations
SET
    title = $1,
    total_tokens_input = $2,
    total_tokens_output = $3,
    summary_turn_id = $4,
    cost_usd = $5,
    last_active = CURRENT_TIMESTAMP
WHERE session_id = $6
RETURNING id, session_id, title, task, goal, start_time, end_time, last_active, status,
    participants, total_turns, total_tokens_input, total_tokens_output, metadata, tags,
    created_at, updated_at, parent_conversation_id, cost_usd, summary_turn_id;

-- name: DeleteSession :exec
DELETE FROM ai_conversations
WHERE session_id = $1;
