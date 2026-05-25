-- name: CreateUser :exec
INSERT INTO users (id, username, password_hash, created_at, updated_at)
VALUES (?, ?, ?, ?, ?);

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = ?;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CreateToken :exec
INSERT INTO tokens (id, user_id, name, hash, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: GetTokenByHash :one
SELECT * FROM tokens WHERE hash = ? AND revoked_at IS NULL;

-- name: TouchToken :exec
UPDATE tokens SET last_used_at = ? WHERE id = ?;

-- name: ListTokensByUser :many
SELECT id, name, last_used_at, created_at, revoked_at
FROM tokens WHERE user_id = ? ORDER BY created_at DESC;

-- name: RevokeToken :exec
UPDATE tokens SET revoked_at = ? WHERE id = ? AND user_id = ?;

-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, expires_at, created_at)
VALUES (?, ?, ?, ?);

-- name: GetSession :one
SELECT * FROM sessions WHERE id = ? AND expires_at > ?;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = ?;

-- name: PurgeExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= ?;
