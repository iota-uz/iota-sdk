-- +migrate Up
-- #1325: core.user.created.v1 notifications move from the eventbus handler to
-- the outbox consumer "notifications". The retained events were already
-- notified by the handler, so they are recorded as delivered to the new
-- consumer instead of being notified a second time. attempts = 0 marks these
-- rows for the Down migration (a real delivery counts at least one attempt).
INSERT INTO extruntime.subscription_receipts (subscription, event_id, tenant_id, status, attempts)
SELECT
    'notifications',
    id,
    tenant_id,
    'done',
    0
FROM
    extruntime.outbox_events
WHERE
    event = 'core.user.created.v1'
ON CONFLICT (subscription, event_id)
    DO NOTHING;

-- +migrate Down
DELETE FROM extruntime.subscription_receipts
WHERE subscription = 'notifications'
    AND attempts = 0;
