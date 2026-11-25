-- +migrate Up
-- Provider updates a connector handles itself (no event is published for them: a /start with a one-use
-- link), claimed before the handler runs and kept with its result, so that two deliveries of one update
-- never both run the handler and a redelivery after a lost answer only finishes what is left (the reply).
CREATE TABLE automation.inbound_actions (
    connection_id text NOT NULL,
    event text NOT NULL,
    dedup_key text NOT NULL,
    -- claimed: the handler is running; handled: it completed and its reply is kept, still to be delivered;
    -- delivering: the reply is being sent; done: finished. A claimed or delivering row older than a few minutes
    -- is abandoned and may be taken over; owner changes with every claim and fences the claimant's writes.
    state text NOT NULL CHECK (state IN ('claimed', 'handled', 'delivering', 'done')),
    owner uuid NOT NULL DEFAULT gen_random_uuid(),
    tenant_id uuid,
    reply text NOT NULL DEFAULT '',
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (connection_id, event, dedup_key)
);

CREATE INDEX inbound_actions_updated_idx ON automation.inbound_actions (updated_at);

-- +migrate Down
DROP TABLE IF EXISTS automation.inbound_actions;
