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
publication/subscription logs. Settings show a single delivery status line:
everything is being delivered, or how many jobs are retrying with a retry action
for managers. Event payloads are not displayed in diagnostics.

## Delivery channels

Rules select channels (`Rule.Channels`: `in_app`, `email`, `sms`, `push`); an empty
selection keeps the previous in-app-only behaviour. The settings page shows the
picker once an adapter is installed. Install providers in the application:

```go
core.NewComponent(&core.ModuleOptions{
    NotificationChannels:        []notifications.ChannelAdapter{emailAdapter, smsAdapter, pushAdapter},
    NotificationAddressResolver: addresses, // email, phone or device token per user
})
```

The SDK ships the contract and one optional provider, [Web Push](#web-push) for
browsers. Applications wire their own email, SMS and native push senders
and decide where addresses and device tokens live. `ChannelAdapter.Send` must
not assume a transaction and should wrap `notifications.ErrPermanent` for errors
that cannot succeed on retry; `AddressResolver` returns `notifications.ErrNoAddress`
when a recipient is unreachable.

Each notice, recipient, channel and target is one row in
`core.notification_deliveries` (see [Recipients, targets and attempts](#recipients-targets-and-attempts)).
In-app delivery records `sent` next to its inbox row and links to it;
read/unread stays on the inbox row. Delivery rows for external channels are
written as intents in the same worker transaction as the in-app write and
checkpoint. Workers then resolve targets, render and call providers with no
transaction open. Failed attempts back off exponentially up to five attempts per
retry budget, then become `failed`. Provider error text and credentials are never
stored: only a short error code. `NotificationChannelService.Retry` reopens one
`failed`, `ambiguous` or `no_address` row without touching other rows. Delivery
is at-least-once: a crash between a provider accepting a message and its outcome
being stored repeats that one message after the two-minute lease expires, and the
interrupted attempt is recorded as `ambiguous`.

A producer that names recipients itself calls
`NotificationDispatchService.EnqueueExplicit` (or `EnqueueAddressed` for the
recipient count, or `EnqueueAddressedTx` with its own `pgx.Tx`). It uses the same
durable queue and per-recipient eligibility checks but ignores the administrator's
rule for the event, so a disabled rule does not suppress it and settings cannot
change its audience or channels. Other deliveries of the same event are unaffected when
one recipient or channel fails. Addressed jobs use their own identity namespace
(`addressed:<id>`), so a rule-based job for the same event ID does not absorb them.
External deliveries run in their own worker loop, so slow providers never delay
in-app dispatch. Eligibility is checked when content is rendered; a recipient who
loses access afterwards can still receive the already rendered message until it
is sent.

### Personal channel choices

Under **Channels per person** (`/settings/notifications/users`) a manager turns a
channel on or off for one person. A channel switched off is skipped for that
person in every event, rule-based or addressed; their other channels and other
recipients are unaffected, and a person with every channel off gets nothing.
Choices are stored as opt-outs in `core.notification_channel_preferences`
(tenant-scoped), so people without a row keep everything the rule selects, and an
opt-out for a channel whose adapter is not installed is kept. The page needs
`NotificationRules.Read`; saving needs `NotificationRules.Manage`.

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

Telegram, administrator-authored expressions/templates,
quiet hours, per-user preferences, retention jobs and durable domain-event replay
remain outside these notification tasks.

## Runtime definition sources

Definitions that do not come from native composition (for example a TypeScript
extension activated from an immutable snapshot) implement
`notifications.DefinitionSource` and are added with `Catalog.RegisterSource`.

- Every definition carries `Source` (the source name) and a non-empty
  `SnapshotRef`. Registration is atomic: one invalid or duplicate definition
  rejects the whole source.
- Keys stay versioned (`...v1`). Registering an existing key fails, so an update
  is published as a new version (`...v2`), never as a silent overwrite.
- Events produced from a runtime definition set `Event.SnapshotRef`. It is part
  of the queued dispatch job's event JSON, so no schema change is needed, and
  `Catalog.For(event)` resolves exactly that snapshot.
- `Catalog.Deactivate(source)` removes the source's definitions from the active
  catalog and the settings list. Pending deliveries keep rendering through their
  snapshot. The catalog is in memory: after a restart the source re-supplies
  the snapshots that still have pending deliveries.
- Definitions are metadata plus simple localized templates; recipients are
  resolved by Go from descriptors. Computing text, payload or explicit recipients
  stays in the author's code before enqueue.

## Recipients, targets and attempts

A notice has four separate identities that the delivery model never mixes:

| Identity | What it is | Where it lives |
|---|---|---|
| Notice | event key/version, snapshot ref, event ID, producer, mode `rule` or `explicit` | `core.notification_notices` |
| Subject | the business object the notice is about (`Event.Subject`, entity + key) | on the notice |
| Recipient | `user`, `record` (entity + key) or `address` (channel → address) | `recipient_*` columns of a delivery |
| Target | the concrete email, phone or push device the recipient is reached on | `target_address`, `target_device_id` |

A customer without an ERP account is a normal recipient: no user row is created
for a contact. In-app delivery exists only for `user` recipients; for `record`
and `address` recipients the in-app channel is ignored, and a send that has such
recipients must select at least one external channel.

### Explicit send

```go
receipt, err := dispatch.EnqueueAddressedTx(ctx, tx, services.AddressedRequest{
    Event: notifications.Event{
        Key: "acme.insurance.renewal.v1", TenantID: tenant,
        SnapshotRef: "pkg-1.4.0",                     // pins the definition and templates
        Subject:     notifications.SubjectReference{Type: "acme.policy", ID: "P-1"},
        Payload:     json.RawMessage(`{"due":"2026-12-01"}`),
    },
    Recipients: []notifications.Addressee{
        notifications.UserAddressee(userID),
        notifications.RecordAddressee("acme.customer", "c-1"),
        notifications.AddressAddressee(map[notifications.Channel]string{notifications.ChannelSMS: "+998901234567"}),
    },
    Channels: []notifications.Channel{notifications.ChannelSMS, notifications.ChannelEmail},
    Producer: notifications.Producer{InstanceID: wf, Occurrence: "remind#2"},
})
```

The notice, its recipients and the dispatch job are written in `tx`, so commit
exposes them to workers and rollback removes them. The context must carry the
tenant. Unknown notices, invalid recipients, a record entity without a registered
`ContactResolver` and unavailable channels fail here, before commit. When the event
has no `DedupeKey`, one is derived from the producer (`InstanceID#Occurrence`, else
`InvocationID`, else `EventID`) plus the subject, recipients, channels and payload,
so a replayed producer step returns the same `NoticeID` while a different intent from
the same producer is its own notice. Rule-mode events (`Enqueue`) keep the
USER/GROUP/ROLE/EVENT_RECIPIENT semantics and are never retargeted by, nor
retarget, an explicit send.

### What happens after commit

1. The dispatch worker writes, per recipient and channel, an `unresolved` intent
   row. For users it also applies eligibility, the personal channel opt-outs and
   `SubjectAccess` (a denial is a `skipped` row with code `subject_access_denied`
   per channel, with no inbox row), and writes the inbox row and its `sent`
   in-app delivery.
2. The channel worker claims intents in a short transaction, then with no
   transaction open expands each into targets:
   - `user`: email/SMS through `AddressResolver`; push through the device
     registry (`core.notification_push_devices`, one delivery per active device),
     falling back to a legacy single token from `AddressResolver` when the user has
     no registered device;
   - `record`: the contact returned by the `ContactResolver` registered for the
     entity, filtered by channel; its language is used for content;
   - `address`: the address itself.
3. Record and address targets pass `DeliveryPolicy` (default allow; deny → `skipped`
   with `policy_denied[:reason]`). The SDK defines no consent schema: the owning
   domain package implements the interface. The platform policy is a
   `*notifications.DeliveryPolicies` composition, see
   [Delivery policy composition](#delivery-policy-composition).
4. The content is rendered once per intent by `ContentRenderers` using the pinned
   `(event key, snapshot ref)`: a renderer registered with `ContentRenderers.Register`
   (the plug-in point for runtime/TypeScript templates), else the catalog definition
   resolved by snapshot, which keeps working after the definition source is
   deactivated or replaced. `ChannelContent` carries title/body/action URL (in-app,
   push), `Data` (push payload), subject/HTML/text (email) and text (SMS). The
   rendered content is persisted with each delivery.
5. Each target is attempted independently by the provider adapter; one target failing
   never changes a sibling's row.

No resolver, policy, renderer or provider is called inside the producer's
transaction. The in-app inbox text is rendered in the dispatch worker's own
transaction (it is written atomically with the checkpoint), so renderers for
in-app content must be fast and local. User eligibility is evaluated when the
intent is written; a recipient who loses access before the provider call still
receives the already queued external message.

### Outcomes

| Delivery status | Meaning |
|---|---|
| `unresolved` | waiting for targets, policy and content (also retrying a resolver outage) |
| `pending` | target known, waiting for an attempt or a scheduled retry |
| `sent` | accepted by the provider (or stored, for in-app) |
| `failed` | permanent provider rejection, or attempts exhausted |
| `ambiguous` | the provider may have accepted it (`ErrAmbiguous`, timeout, lease expiry); never retried automatically |
| `no_address` | no address or device for this recipient and channel (`contact_not_found` for an unknown record) |
| `skipped` | access or business policy stopped it; the error code says which |

`core.notification_delivery_attempts` records every provider call: attempt number,
start and finish, outcome `sent|failed|permanent|ambiguous`, the provider receipt and
an error code. `Message.IdempotencyKey` is the delivery ID and is the same on every
attempt of a delivery. Adapters return `notifications.ErrPermanent` for a rejection
and `notifications.ErrAmbiguous` when the outcome is unknown. A target or
`no_address` row reopened with `Retry` gets a fresh attempt budget; rows with
no target go back through resolution.

### Wiring

```go
core.NewComponent(&core.ModuleOptions{
    NotificationChannels:         []notifications.ChannelAdapter{email, sms, push},
    NotificationAddressResolver:  userAddresses,                      // users' email and phone
    NotificationContactResolvers: map[string]notifications.ContactResolver{"acme.customer": customers},
    NotificationSubjectAccess:    subjectAccess,                      // may user U see subject ref?
    NotificationDeliveryPolicy:   consentPolicy,                      // business consent, default allow
    NotificationWebPush:          webPushOptions,                     // optional, off by default; see "Web Push"
})
```

Push devices of users are managed with `NotificationChannelService.RegisterDevice` and
`RevokeDevice` (a token that another registration of the tenant holds moves to the
newest). External recipients bring their devices through their `ContactResolver`. The
extension runtime can register more resolvers at runtime on the
`*notifications.ContactResolvers` registry and renderers on
`*notifications.ContentRenderers`, both resolvable from the container.

Synthetic providers for tests live in `modules/core/notifications/notificationstest`
(recording email/SMS/push adapters with per-address failure, ambiguous and permanent
outcomes, a map-backed contact resolver, recording access and policy hooks). They are
never registered by the core component.

### Delivery policy composition

The notification platform consults one `DeliveryPolicy`: the `*notifications.DeliveryPolicies`
registry (resolvable from the container). Its members are evaluated in a fixed order:
the policy configured in Go (`ModuleOptions.NotificationDeliveryPolicy`) first, then the
named policies registered with `DeliveryPolicies.Set(name, policy)` in lexical order of
their names. **Any denial wins**: the first denial in that order ends the walk and its
reason is the one stored. A member that returns an error fails the whole evaluation, so
the delivery stays `unresolved` and is retried rather than sent without the verdict.
`Set` replaces a policy of the same name and a nil policy removes it.

The extension runtime installs one member, `extruntime`
(`pkg/extruntime/notify.PolicyName`). For a record recipient it runs the
`deliveryPolicy` implementations of the **active** package versions bound to the
recipient's entity (ordered by policy ID, then package), each as the trigger owner of
the policy, outside any transaction and after the producer committed. Unbound or
deactivated policies are not consulted. The wire request and answer are documented in the
authoring guide (*Delivery policies*) and are a public contract.

## Web Push

`modules/core/notifications/webpush` is a `ChannelAdapter` for the push channel that
sends to browsers over the Web Push protocol itself: RFC 8291 message encryption
(aes128gcm) and RFC 8292 VAPID, with the Go standard library plus the JWT library the
SDK already depends on. It was written rather than taken from a library because the
protocol is small, the cryptography is standard-library primitives, a new dependency
would be one more thing to audit for a path that handles user subscriptions, and the
encryption is pinned byte-for-byte to the RFC 8291 test vector in the tests.

It is **off by default**. Enable it with options built from secrets:

```go
opts, enabled, err := webpush.FromEnv(os.LookupEnv) // enabled is false when no key variable is set
if err != nil { /* a half-configured deployment: fail startup */ }
modOpts := &core.ModuleOptions{}
if enabled {
    modOpts.NotificationWebPush = &opts
}
core.NewComponent(modOpts)
```

| Variable | |
|---|---|
| `WEB_PUSH_VAPID_PUBLIC_KEY`, `WEB_PUSH_VAPID_PRIVATE_KEY` | base64url key pair; create it once with `webpush.GenerateKeys()` and keep the private key in the secret store |
| `WEB_PUSH_SUBJECT` | `mailto:` or `https:` contact for the push service |
| `WEB_PUSH_ALLOWED_HOSTS` | optional comma-separated host patterns; default is the major browsers' push services |
| `WEB_PUSH_TTL` | optional duration, default `24h` |

Setting only some of the key variables fails startup. When enabled the component installs the
adapter and the signed-in user's registration API:

| Route | |
|---|---|
| `GET /notifications/push/config` | `{"vapidPublicKey": "..."}` for `pushManager.subscribe({applicationServerKey})` |
| `PUT /notifications/push/devices` | `{"subscription": <PushSubscription.toJSON()>}` → `{"deviceId": "web:…"}`; calls `RegisterDevice` |
| `DELETE /notifications/push/devices/{deviceId}` | `RevokeDevice`; 204 |

The recipient is always the authenticated user. The device ID derives from the endpoint, so
registering the same subscription again refreshes one device; the device token is the
subscription JSON. Endpoints are only accepted on allowed push-service hosts and over
https (users choose the endpoint, so this is what stops the server being used to call
arbitrary hosts), and redirects are never followed.

Outcomes: `201/202` → `sent`; `404/410` → `failed` and the device is revoked
(`notifications.ErrTargetGone`); other `4xx` → `failed`; `408/429/5xx` and transport
errors → retried; a timeout after sending → `ambiguous`. The delivery ID becomes the
`Topic` header so a retried message on an offline device is shown once. Native iOS/Android
tokens are not covered; install an adapter for them in `NotificationChannels`.
