-- Index the foreign keys of the integrations page tables (#1447): deleting a user or a connection
-- cascades into these tables, and without an index on the referencing column it scans them.
-- connection_id is NULL for most rows, so its indexes are partial.
-- +migrate Up
CREATE INDEX oauth_states_user_id_idx ON automation.oauth_states (user_id);

CREATE INDEX oauth_states_connection_id_idx ON automation.oauth_states (connection_id)
WHERE
    connection_id IS NOT NULL;

CREATE INDEX connection_requests_connection_id_idx ON automation.connection_requests (connection_id)
WHERE
    connection_id IS NOT NULL;

-- +migrate Down
DROP INDEX IF EXISTS automation.connection_requests_connection_id_idx;

DROP INDEX IF EXISTS automation.oauth_states_connection_id_idx;

DROP INDEX IF EXISTS automation.oauth_states_user_id_idx;
