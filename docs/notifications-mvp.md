# Back-office notifications

Core provides persistent notifications, the navbar bell with the latest ten
messages, cursor-paginated history at `/notifications`, and administrator controls
at `/settings/notifications`. This completes the notification feature following
#216. PostgreSQL is authoritative; WebSocket events refresh the badge, dropdown,
and open history and show the SDK toast matching the notification level.
Reconnect and reload rebuild the UI from persistence.

## Rollout

1. Apply SDK migrations before deploying using the target database configuration:
   `go run cmd/command/main.go migrate up`. The notification migrations create
   inboxes, rules, their level/audit fields, and durable dispatch jobs.
2. Grant `NotificationRules.Read` and `NotificationRules.Manage` to pilot
   administrators through the application's permission provisioning. Custom
   permission schemas must include these permissions. Read-only administrators
   may inspect rules and queue status; only managers may save rules or retry jobs.
3. For multiple replicas configure the same `REDIS_URL` on every instance.
   Without it realtime supports one instance. A configured Redis that cannot
   connect or subscribe prevents startup; see [realtime fanout](notifications-realtime.md).
4. In settings enable **Test notification**, select users, groups or roles, choose
   a level, and save. **Send test using saved rule** reports the accepted audience
   size. Delivery is asynchronous; the queue shows processed/delivered counts and
   scheduled retries. Verify the recipient's badge, toast, dropdown and history
   in another tab or replica, then mark messages read.
5. Enable **User created** for recipients with `User.Read`, including supported
   event participants if desired. Create a user normally and verify its localized
   notification and internal link. Disable the rule and verify new events stop.

Rules union explicit users, direct group members, effective roles (including
roles inherited through groups), and the semantic participants registered by the
event. Missing references stay visible until removed. Only active, unblocked,
ordinary users in the current tenant receive messages. Event permissions and
optional object guards are checked at delivery. Settings validate tenant-owned
references and reject unsupported semantic keys.

## Application event contract

Register definitions through `core.ModuleOptions.NotificationEvents`. Definitions
have stable versioned keys, localized module metadata, default levels, documented
payload fields, and supported semantic recipient resolvers. Optional extensions
include safe default semantic recipients, payload validation, an internal action
URL builder, a renderer and an object-level recipient guard. Duplicate definitions
fail composition; unknown events, version mismatches, missing required fields and
invalid payloads fail publication. Applications own object-specific permissions
and payload policy.

Resolve `*services.NotificationDispatchService` and call `Enqueue(ctx, event)`
with database pool and matching tenant in context. The envelope contains `Key`,
`Version`, stable `ID`, `TenantID`, optional `ActorUserID`, `OccurredAt`, `Subject`,
`DedupeKey` and validated `Data`. For compatibility, omitted version/time are
normalized to the registered version/current UTC time. Enqueue can share a
business transaction: commit exposes the job to workers and rollback removes it.
A stable event identity deduplicates jobs and per-recipient inbox rows; when
present, `DedupeKey` takes precedence over `ID` within the event key and tenant.

The synchronous `NotificationRoutingService.Publish` remains available for small
trusted producers and accepts an injected `NotificationDelivery` implementation.
Use the durable dispatch service for large audiences. The trusted
`NotificationService.Deliver` supports explicit recipients. When using a
caller-owned transaction with direct delivery, call `NotifyCommitted` only after
commit; the dispatch worker handles this automatically. Recipient-facing APIs
always derive the recipient from authenticated context.

## Durable dispatch and failure semantics

Enqueue freezes the event payload, saved rule and expanded recipient IDs. Later
membership/rule changes affect new events. Workers recheck current eligibility
and authorization, then render in each recipient's current language; persisted
text does not change afterward. Explicitly disabled rules suppress new jobs;
events with safe default participants may run before an administrator configures
them. An explicitly saved rule takes precedence over defaults.

Core starts a worker on API/worker compositions unless
`core.ModuleOptions.DisableNotificationWorker` is set. Each tick processes one
batch of up to 100 candidates per active tenant. Replicas claim jobs with
`FOR UPDATE SKIP LOCKED`. Notification writes and the checkpoint commit in one
transaction, so a crash resumes at the last committed checkpoint. A failed batch
rolls back, records a generic diagnostic, and retries with exponential backoff
capped at five minutes. Managers can request an immediate retry in settings.
Completed jobs and inboxes have no automatic retention policy.

Realtime publication happens after commit. Redis failures are logged and leave
persisted notifications intact; Pub/Sub has no replay. The UI recovers on
reconnect or reload. Existing broad realtime handlers also check tenant and read
permissions before sending authenticated fragments.

The built-in user-created handler has a five-second enqueue deadline. Inside a
caller-owned transaction it uses a savepoint, so an enqueue failure is logged
without aborting user creation. There is no durable domain-event outbox: a crash
between business commit and enqueue can lose an event. Producers requiring that
guarantee must enqueue in their business transaction or use an application outbox
and retry a stable identity. Queue durability starts once the job commits.

Monitor `failed to persist user-created notifications`,
`notification delivery batch failed; durable retry scheduled`, and realtime
publication/subscription logs. Settings expose pending, retrying and completed
counts plus the latest 25 jobs. Event payloads are not displayed in diagnostics.

## Validation

```sh
go vet ./...
go test ./modules/core/domain/aggregates/notification ./modules/core/notifications
go test ./modules/core/infrastructure/persistence ./modules/core/services \
  ./modules/core/presentation/controllers ./modules/core \
  -run 'TestNotification|TestComponent' -count=1
go test -race ./pkg/realtime/... ./pkg/application ./pkg/bootstrap ./pkg/ws
```

Browser acceptance is `e2e/tests/core/notifications.spec.ts`. Run against an
isolated seeded SDK server using `BASE_URL`; set `SECONDARY_BASE_URL` to another
SDK replica with the same PostgreSQL and Redis to check cross-instance delivery.
It covers saved settings, overlapping/group/role audiences, severity toast,
realtime dropdown/history, read persistence, membership removal and disabling.

Email, SMS, Telegram, browser push, administrator-authored expressions/templates,
quiet hours, per-user preferences, retention jobs and durable domain-event replay
remain outside these notification tasks.
