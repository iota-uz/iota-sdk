-- The memory of an automation between its runs (#1390): key-value pairs per automation and tenant that
-- the set_state node writes and the CEL variable state reads (state.get(key), state.has(key)). A key with
-- expires_at is no longer seen once it passes and a background sweep removes it.
-- +migrate Up
CREATE TABLE automation.state (
    tenant_id uuid NOT NULL,
    automation_id uuid NOT NULL,
    key text NOT NULL CHECK (char_length(key) BETWEEN 1 AND 200),
    value jsonb NOT NULL,
    expires_at timestamp with time zone,
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    -- The run whose step wrote the value; null for an edit by a person or an agent.
    updated_by_run uuid,
    PRIMARY KEY (tenant_id, automation_id, key),
    FOREIGN KEY (tenant_id, automation_id) REFERENCES automation.automations (tenant_id, id) ON DELETE CASCADE
);

-- The sweep of expired keys.
CREATE INDEX state_expires_idx ON automation.state (expires_at)
WHERE
    expires_at IS NOT NULL;

-- +migrate Down
DROP TABLE IF EXISTS automation.state;
