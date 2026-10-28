-- +migrate Up
CREATE TABLE business_event_deliveries (
    tenant_id uuid NOT NULL,
    event_id uuid NOT NULL,
    consumer varchar(200) NOT NULL,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error text NOT NULL DEFAULT '',
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    available_at timestamp with time zone NOT NULL DEFAULT now(),
    delivered_at timestamp with time zone,
    PRIMARY KEY (tenant_id, event_id, consumer),
    FOREIGN KEY (tenant_id, event_id) REFERENCES business_events (tenant_id, event_id) ON DELETE CASCADE
);

CREATE INDEX business_event_deliveries_due_idx ON business_event_deliveries (available_at, created_at)
WHERE
    delivered_at IS NULL;

-- +migrate Down
DROP TABLE IF EXISTS business_event_deliveries;
