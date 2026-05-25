-- +goose Up
-- +goose StatementBegin

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    hash TEXT NOT NULL UNIQUE,
    last_used_at INTEGER,
    created_at INTEGER NOT NULL,
    revoked_at INTEGER
);

CREATE INDEX idx_tokens_user ON tokens(user_id);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE INDEX idx_sessions_user ON sessions(user_id);

CREATE TABLE apps (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    image_repo TEXT NOT NULL DEFAULT '',
    current_version TEXT NOT NULL DEFAULT '',
    instances INTEGER NOT NULL DEFAULT 1,
    deploy_strategy TEXT NOT NULL DEFAULT 'blue_green',
    healthcheck_path TEXT NOT NULL DEFAULT '',
    healthcheck_cmd TEXT NOT NULL DEFAULT '',
    internal_only INTEGER NOT NULL DEFAULT 0,
    memory_mb INTEGER NOT NULL DEFAULT 0,
    cpu_pct INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE app_versions (
    id TEXT PRIMARY KEY,
    app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    version TEXT NOT NULL,
    image_tag TEXT NOT NULL,
    status TEXT NOT NULL,
    log_excerpt TEXT NOT NULL DEFAULT '',
    deployed_at INTEGER NOT NULL
);

CREATE INDEX idx_app_versions_app ON app_versions(app_id);

CREATE TABLE app_env (
    id TEXT PRIMARY KEY,
    app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value_encrypted BLOB NOT NULL,
    UNIQUE(app_id, key)
);

CREATE TABLE app_volumes (
    id TEXT PRIMARY KEY,
    app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    container_path TEXT NOT NULL,
    host_path TEXT NOT NULL DEFAULT '',
    named_volume TEXT NOT NULL DEFAULT ''
);

CREATE TABLE app_ports (
    id TEXT PRIMARY KEY,
    app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    container_port INTEGER NOT NULL,
    protocol TEXT NOT NULL DEFAULT 'tcp'
);

CREATE TABLE app_domains (
    id TEXT PRIMARY KEY,
    app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    domain TEXT NOT NULL UNIQUE,
    is_primary INTEGER NOT NULL DEFAULT 0,
    tls_state TEXT NOT NULL DEFAULT 'pending'
);

CREATE TABLE templates_installed (
    id TEXT PRIMARY KEY,
    template_id TEXT NOT NULL,
    app_name TEXT NOT NULL,
    params_json TEXT NOT NULL,
    installed_at INTEGER NOT NULL
);

CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE audit_log (
    id TEXT PRIMARY KEY,
    user_id TEXT,
    action TEXT NOT NULL,
    target TEXT NOT NULL DEFAULT '',
    payload_json TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS templates_installed;
DROP TABLE IF EXISTS app_domains;
DROP TABLE IF EXISTS app_ports;
DROP TABLE IF EXISTS app_volumes;
DROP TABLE IF EXISTS app_env;
DROP TABLE IF EXISTS app_versions;
DROP TABLE IF EXISTS apps;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS tokens;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
