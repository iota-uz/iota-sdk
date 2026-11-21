-- +migrate Up
-- Integrations page (#1334): one-time OAuth2 states and the time each connection was last checked.

-- A state from OAuth2 Begin is accepted once, by the user and tenant that started it, for its kind,
-- until expires_at. Only its SHA-256 is kept.
CREATE TABLE automation.oauth_states (
    state_hash bytea PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id bigint NOT NULL,
    kind text NOT NULL,
    name text NOT NULL DEFAULT '',
    display_name text NOT NULL DEFAULT '',
    connection_id uuid,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX oauth_states_expires_at_idx ON automation.oauth_states (expires_at);

CREATE TABLE automation.connection_checks (
    connection_id uuid PRIMARY KEY REFERENCES automation.connections (id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    checked_at timestamp with time zone NOT NULL
);

-- +migrate Down
DROP TABLE IF EXISTS automation.connection_checks;

DROP TABLE IF EXISTS automation.oauth_states;
