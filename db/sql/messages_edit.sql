-- name: GetUserMessages :many
SELECT *
FROM messages
WHERE session_id = ?
AND role = 'user'
ORDER BY created_at DESC;

-- name: GetMessagesAfter :many
SELECT m1.*
FROM messages m1
WHERE m1.session_id = ?
AND m1.created_at > (SELECT m2.created_at FROM messages m2 WHERE m2.id = ?)
ORDER BY m1.created_at ASC;

-- name: DeleteMessagesAfter :exec
DELETE FROM messages
WHERE id IN (
    SELECT m1.id
    FROM messages m1
    WHERE m1.session_id = ?
    AND m1.created_at > (SELECT m2.created_at FROM messages m2 WHERE m2.id = ?)
);

-- name: DeleteMessageByID :exec
DELETE FROM messages
WHERE id = ?;

-- name: CountMessagesAfter :one
SELECT COUNT(*) as count
FROM messages m1
WHERE m1.session_id = ?
AND m1.created_at > (SELECT m2.created_at FROM messages m2 WHERE m2.id = ?);
