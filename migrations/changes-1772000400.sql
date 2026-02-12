-- +migrate Up
-- Applet engine secrets store (slice 2)

CREATE TABLE IF NOT EXISTS applet_engine_secrets (
    applet_id TEXT NOT NULL,
    secret_name TEXT NOT NULL,
    cipher_text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (applet_id, secret_name)
);

-- +migrate Down
DROP TABLE IF EXISTS applet_engine_secrets;
