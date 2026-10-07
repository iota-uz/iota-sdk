# In-app notifications MVP

The core component provides persistent notifications, a notification center at
`/notifications`, and administrator controls at `/settings/notifications`.
The navbar count and open notification history refresh every 20 seconds from
PostgreSQL. All replicas read the same database; Redis is not required.

## Rollout

1. Apply SDK migrations before deploying the updated application:
   `go run cmd/command/main.go migrate up` using the target database configuration.
   `changes-1791300000.sql` creates `core.notifications`,
   `core.notification_rules` with user, group, and role selections, their indexes,
   and two settings permissions.
2. Grant `NotificationRules.Read` and `NotificationRules.Manage` to the pilot
   administrator through the application's permission provisioning process.
   The default SDK permission schema exposes separate read and manage sets.
   Existing users receive no new administrative rights automatically. Applications
   with a custom permission schema must include these permissions in that schema.
3. Open notification settings, enable **Test notification**, select users, groups or roles,
   and save. Click **Send test using saved rule**. The result reports the recipient
   count; disabled rules send nothing. Open another recipient tab and wait up to
   20 seconds for its badge/history to update.
4. Enable **User created**, select recipients who have `User.Read`, and create
   a user normally. Recipients receive a localized snapshot with a link to that
   user. Mark one notification read, reload, then mark all read. Disable the
   event and verify further user creation produces no notifications.

Rules combine selected users, groups, and roles without duplicate deliveries. Roles
include direct assignments and roles inherited through groups. Group membership
and effective roles are resolved for each event; later membership changes affect
new deliveries, while notification history remains unchanged. Empty groups and
roles may be configured before members are assigned. Deleted audiences are shown
as unavailable in settings and must be removed when saving.

Only active, unblocked ordinary users of the current tenant receive notifications.
Required event permissions are checked again at delivery. Deleted, blocked,
inactive, or unauthorized recipients are skipped. Recipient membership and text
are snapshots; later account or language changes do not rewrite history.

## Adding application events

Pass definitions in `core.ModuleOptions.NotificationEvents`. Each definition has
a stable versioned key, localized name/description, an optional required permission,
and a renderer returning title, body, and an internal action URL. Duplicate keys
fail during composition. Definitions should render only information permitted by
their required permission; object-specific authorization remains the producer's
responsibility in this MVP.

Resolve `*services.NotificationRoutingService` from composition and call
`Publish(ctx, notifications.Event{Key: key, ID: stableEventID, TenantID: tenantID,
Data: payload})`. The context must contain the database pool and matching tenant.
Publish after the business transaction commits. A stable event ID deduplicates
repeated delivery for each recipient; a new ID creates a new notification.
Notifications are off until the tenant administrator configures a rule.

The trusted `NotificationService.Deliver` API also supports direct user delivery.
Recipient-facing service APIs derive the user from authenticated context and
cannot read or mutate another user's notifications.

## Operating limits

Delivery errors are returned by the router. The built-in user-created event
handler logs failures with tenant and user IDs without failing user creation.
When the producer uses a caller-owned transaction, notification writes run in a
savepoint: successful writes share the caller's commit or rollback, while SQL
failures roll back notification writes without aborting user creation.
The built-in handler has a five-second deadline and no durable delivery queue.
Large group or role audiences can exceed that deadline; outside a caller-owned
transaction, already persisted notifications remain while later recipients may
be omitted. Durable audience snapshots, batched delivery, and resumable retries
remain outside this MVP and are required before a large-audience rollout.
There is no durable event outbox/replay: an event may be lost if the process
exits between business commit and notification persistence. Producers requiring
guaranteed delivery should retry using a stable ID or add an outbox in their
application. A persisted notification survives reloads and replica changes.

Monitor `failed to persist user-created notifications` logs and database errors.
Rules have no automatic retention; history grows until an application establishes
its retention policy. Each visible tab polls at most two small endpoints every
20 seconds (count plus history while the center is open).

This slice omits custom dynamic audience predicates, external delivery channels,
WebSocket fanout, custom templates, severity overrides, per-user quiet hours,
and retention jobs. The existing broader issues remain open for these extensions.

## Validation

```sh
go vet ./...
go test ./modules/core/domain/aggregates/notification ./modules/core/notifications
go test ./modules/core/infrastructure/persistence ./modules/core/services \
  ./modules/core/presentation/controllers ./modules/core \
  -run 'TestNotification|TestComponent' -count=1
templ generate
just css
```

The focused tests cover persistent recipient/tenant isolation, deduplication,
URL validation, read state, settings authorization, routing, and an actual
user-creation-to-inbox path. The user-created handler retains a caller-owned
transaction, so rolling back
user creation also rolls back its notifications.

Browser acceptance is in
`e2e/tests/core/notifications.spec.ts`; run against an isolated seeded SDK server
using the configured `BASE_URL`. It checks saved settings, delivery to a second
tab, read-state persistence, saved group/role selections, inherited-role delivery,
membership removal, and disabling delivery.
