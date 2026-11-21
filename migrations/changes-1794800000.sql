-- +migrate Up
-- #1325: core.user.created.v1 notifications move from the eventbus handler to
-- the outbox consumer "notifications". A retained event the handler already
-- turned into a notification (its dispatch job or notice, identity
-- "user-created:<user ID>") is recorded as delivered to the new consumer, so
-- it is not notified twice. A retained event the handler never enqueued (the
-- process died between the commit and the eventbus publish, or the enqueue
-- failed) gets no receipt and is delivered by the consumer. attempts = 0 marks
-- the rows of this migration for the Down migration (a real delivery counts at
-- least one attempt).
INSERT INTO extruntime.subscription_receipts (subscription, event_id, tenant_id, status, attempts)
SELECT
    'notifications',
    o.id,
    o.tenant_id,
    'done',
    0
FROM
    extruntime.outbox_events o
WHERE
    o.event = 'core.user.created.v1'
    AND o.subject_key IS NOT NULL
    AND (EXISTS (
            SELECT
                1
            FROM
                core.notification_dispatches d
            WHERE
                d.tenant_id = o.tenant_id
                AND d.event_key = o.event
                AND d.event_id = 'user-created:' || o.subject_key)
            OR EXISTS (
                SELECT
                    1
                FROM
                    core.notification_notices n
                WHERE
                    n.tenant_id = o.tenant_id
                    AND n.event_key = o.event
                    AND n.event_id = 'user-created:' || o.subject_key))
ON CONFLICT (subscription, event_id)
    DO NOTHING;

-- +migrate Down
DELETE FROM extruntime.subscription_receipts
WHERE subscription = 'notifications'
    AND attempts = 0;
