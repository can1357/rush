-- name: AddPromptHistory :one
INSERT INTO prompt_history (
    id,
    prompt,
    created_at
) VALUES (
    ?,
    ?,
    strftime('%s', 'now')
)
ON CONFLICT(prompt) DO UPDATE SET
    created_at = strftime('%s', 'now')
RETURNING *;

-- name: ListPromptHistory :many
SELECT *
FROM prompt_history
ORDER BY created_at DESC
LIMIT ?;

-- name: GetPromptHistoryCount :one
SELECT COUNT(*) as count
FROM prompt_history;

-- name: ClearPromptHistory :exec
DELETE FROM prompt_history;

-- name: GetPromptByID :one
SELECT *
FROM prompt_history
WHERE id = ? LIMIT 1;
