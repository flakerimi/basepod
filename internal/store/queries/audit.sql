-- name: ListAuditLog :many
SELECT * FROM audit_log
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListAuditLogForTarget :many
SELECT * FROM audit_log
WHERE target = ?
ORDER BY created_at DESC
LIMIT ?;
