-- name: CreateTodo :one
INSERT INTO todos (
    id,
    session_id,
    content,
    active_form,
    status,
    position,
    created_at,
    updated_at,
    completed_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    strftime('%s', 'now') * 1000,
    strftime('%s', 'now') * 1000,
    NULL
) RETURNING *;

-- name: GetTodoByID :one
SELECT *
FROM todos
WHERE id = ? LIMIT 1;

-- name: ListTodosBySession :many
SELECT *
FROM todos
WHERE session_id = ?
ORDER BY position ASC;

-- name: UpdateTodoStatus :one
UPDATE todos
SET
    status = ?,
    completed_at = CASE
        WHEN ? = 'completed' AND completed_at IS NULL THEN strftime('%s', 'now') * 1000
        WHEN ? != 'completed' THEN NULL
        ELSE completed_at
    END
WHERE id = ?
RETURNING *;

-- name: UpdateTodoContent :one
UPDATE todos
SET
    content = ?,
    active_form = ?
WHERE id = ?
RETURNING *;

-- name: DeleteTodo :exec
DELETE FROM todos
WHERE id = ?;

-- name: DeleteTodosBySession :exec
DELETE FROM todos
WHERE session_id = ?;

-- name: UpdateTodoPosition :exec
UPDATE todos
SET position = ?
WHERE id = ?;

-- name: GetMaxTodoPosition :one
SELECT COALESCE(MAX(position), -1) as max_position
FROM todos
WHERE session_id = ?;
