-- name: GetFile :one
SELECT *
FROM files
WHERE id = $1 LIMIT 1;

-- name: GetFileByPathAndSession :one
SELECT f.*
FROM files f
JOIN ai_conversations c ON f.conversation_id = c.id
WHERE f.path = $1 AND c.session_id = $2
ORDER BY f.version DESC, f.created_at DESC
LIMIT 1;

-- name: ListFilesBySession :many
SELECT f.*
FROM files f
JOIN ai_conversations c ON f.conversation_id = c.id
WHERE c.session_id = $1
ORDER BY f.version ASC, f.created_at ASC;

-- name: ListFilesByPath :many
SELECT *
FROM files
WHERE path = $1
ORDER BY version DESC, created_at DESC;

-- name: CreateFile :one
INSERT INTO files (
    id,
    conversation_id,
    path,
    content,
    version
)
SELECT
    $1::uuid,
    c.id,
    $3,
    $4,
    $5
FROM ai_conversations c
WHERE c.session_id = $2
RETURNING *;

-- name: DeleteFile :exec
DELETE FROM files
WHERE id = $1;

-- name: DeleteSessionFiles :exec
DELETE FROM files f
USING ai_conversations c
WHERE f.conversation_id = c.id
AND c.session_id = $1;

-- name: ListLatestSessionFiles :many
SELECT f.*
FROM files f
JOIN ai_conversations c ON f.conversation_id = c.id
JOIN (
    SELECT path, MAX(version) as max_version
    FROM files
    GROUP BY path
) latest ON f.path = latest.path AND f.version = latest.max_version
WHERE c.session_id = $1
ORDER BY f.path;

-- name: ListNewFiles :many
SELECT *
FROM files
WHERE version = 0
ORDER BY created_at DESC;
