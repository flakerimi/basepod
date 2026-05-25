-- name: CreateApp :exec
INSERT INTO apps (
    id, name, image_repo, current_version, instances, deploy_strategy,
    healthcheck_path, healthcheck_cmd, internal_only, memory_mb, cpu_pct,
    created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAppByName :one
SELECT * FROM apps WHERE name = ?;

-- name: GetAppByID :one
SELECT * FROM apps WHERE id = ?;

-- name: ListApps :many
SELECT * FROM apps ORDER BY name;

-- name: UpdateApp :exec
UPDATE apps SET
    image_repo = ?,
    current_version = ?,
    instances = ?,
    deploy_strategy = ?,
    healthcheck_path = ?,
    healthcheck_cmd = ?,
    internal_only = ?,
    memory_mb = ?,
    cpu_pct = ?,
    updated_at = ?
WHERE id = ?;

-- name: SetAppCurrentVersion :exec
UPDATE apps SET current_version = ?, image_repo = ?, updated_at = ? WHERE id = ?;

-- name: DeleteApp :exec
DELETE FROM apps WHERE id = ?;

-- name: CreateAppVersion :exec
INSERT INTO app_versions (id, app_id, version, image_tag, status, log_excerpt, deployed_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListAppVersions :many
SELECT * FROM app_versions WHERE app_id = ? ORDER BY deployed_at DESC;

-- name: UpdateAppVersionStatus :exec
UPDATE app_versions SET status = ?, log_excerpt = ? WHERE id = ?;

-- name: ListAppVersionIDsToPrune :many
SELECT id FROM app_versions
WHERE app_id = ?
ORDER BY deployed_at DESC
LIMIT -1 OFFSET ?;

-- name: DeleteAppVersionByID :exec
DELETE FROM app_versions WHERE id = ?;

-- name: UpsertAppEnv :exec
INSERT INTO app_env (id, app_id, key, value_encrypted) VALUES (?, ?, ?, ?)
ON CONFLICT(app_id, key) DO UPDATE SET value_encrypted = excluded.value_encrypted;

-- name: ListAppEnv :many
SELECT id, key, value_encrypted FROM app_env WHERE app_id = ? ORDER BY key;

-- name: DeleteAppEnv :exec
DELETE FROM app_env WHERE app_id = ? AND key = ?;

-- name: ClearAppEnv :exec
DELETE FROM app_env WHERE app_id = ?;

-- name: AddAppVolume :exec
INSERT INTO app_volumes (id, app_id, container_path, host_path, named_volume)
VALUES (?, ?, ?, ?, ?);

-- name: ListAppVolumes :many
SELECT * FROM app_volumes WHERE app_id = ?;

-- name: ClearAppVolumes :exec
DELETE FROM app_volumes WHERE app_id = ?;

-- name: AddAppPort :exec
INSERT INTO app_ports (id, app_id, container_port, protocol)
VALUES (?, ?, ?, ?);

-- name: ListAppPorts :many
SELECT * FROM app_ports WHERE app_id = ?;

-- name: ClearAppPorts :exec
DELETE FROM app_ports WHERE app_id = ?;

-- name: DeleteAppPort :exec
DELETE FROM app_ports WHERE app_id = ? AND container_port = ?;

-- name: DeleteAppVolumeByPath :exec
DELETE FROM app_volumes WHERE app_id = ? AND container_path = ?;

-- name: AddAppDomain :exec
INSERT INTO app_domains (id, app_id, domain, is_primary, tls_state)
VALUES (?, ?, ?, ?, ?);

-- name: ListAppDomains :many
SELECT * FROM app_domains WHERE app_id = ? ORDER BY is_primary DESC, domain;

-- name: ListAllDomains :many
SELECT * FROM app_domains ORDER BY domain;

-- name: DeleteAppDomain :exec
DELETE FROM app_domains WHERE app_id = ? AND domain = ?;

-- name: SetAppGit :exec
UPDATE apps SET
    git_url = ?,
    git_branch = ?,
    git_dockerfile = ?,
    git_token_encrypted = ?,
    updated_at = ?
WHERE id = ?;

-- name: SetAppWebhookSecret :exec
UPDATE apps SET webhook_secret_encrypted = ?, updated_at = ? WHERE id = ?;

-- name: GetAppGit :one
SELECT git_url, git_branch, git_dockerfile, git_token_encrypted, webhook_secret_encrypted
FROM apps WHERE id = ?;

-- name: GetSetting :one
SELECT value FROM settings WHERE key = ?;

-- name: UpsertSetting :exec
INSERT INTO settings (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value;

-- name: ListSettings :many
SELECT key, value FROM settings;

-- name: InsertAuditLog :exec
INSERT INTO audit_log (id, user_id, action, target, payload_json, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: RecordTemplateInstall :exec
INSERT INTO templates_installed (id, template_id, app_name, params_json, installed_at)
VALUES (?, ?, ?, ?, ?);

-- name: ListTemplatesInstalled :many
SELECT * FROM templates_installed ORDER BY installed_at DESC;
