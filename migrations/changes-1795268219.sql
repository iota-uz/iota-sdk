-- +migrate Up
-- Integrations page (#1334): one-time OAuth2 states, the time each connection was last checked and the
-- idempotency keys of the page's requests.

-- A state from OAuth2 Begin is accepted once, by the user and tenant that started it, for its kind,
-- until expires_at. Only its SHA-256 is kept.
CREATE TABLE automation.oauth_states (
    state_hash bytea PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind text NOT NULL,
    name text NOT NULL DEFAULT '',
    display_name text NOT NULL DEFAULT '',
    connection_id uuid REFERENCES automation.connections (id) ON DELETE CASCADE,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX oauth_states_expires_at_idx ON automation.oauth_states (expires_at);

CREATE INDEX oauth_states_tenant_id_idx ON automation.oauth_states (tenant_id);

CREATE TABLE automation.connection_checks (
    connection_id uuid PRIMARY KEY REFERENCES automation.connections (id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    checked_at timestamp with time zone NOT NULL
);

CREATE INDEX connection_checks_tenant_id_idx ON automation.connection_checks (tenant_id);

-- The connection an idempotency key of the integrations page produced: a retry of a request whose
-- answer was lost returns that connection instead of creating or changing it again. Kept a day.
CREATE TABLE automation.connection_requests (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    key text NOT NULL CHECK (key <> '' AND length(key) <= 100),
    connection_id uuid NOT NULL REFERENCES automation.connections (id) ON DELETE CASCADE,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, key)
);

CREATE INDEX connection_requests_created_at_idx ON automation.connection_requests (created_at);

-- +migrate Down
DROP TABLE IF EXISTS automation.connection_requests;

DROP TABLE IF EXISTS automation.connection_checks;

DROP TABLE IF EXISTS automation.oauth_states;
