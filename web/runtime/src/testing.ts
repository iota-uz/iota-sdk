// Test support for extension packages (`@iota-uz/sdk/runtime/testing`): synthetic typed fixtures, fake
// external effects and an in-memory host that runs handlers without Granite or a database.
export { collectManifest, ManifestError } from './manifest.ts'
export type { CollectInput } from './manifest.ts'
export { EPOCH, OVERLAY_OF, fixture, overlayFixture } from './fixtures.ts'
export type { Fixture } from './fixtures.ts'
export { UnmatchedRequestError, fakeEffects } from './fakes.ts'
export type {
  EmailMessage, FakeChannel, FakeEffects, FakeHttp, FakeHttpRequest, FakeHttpResponse, PushMessage, SmsMessage,
} from './fakes.ts'
export { TestHostError, createTestHost, describe, test } from './testhost.ts'
export type {
  EmittedEvent, PendingWork, SentNotice, StartedWorkflow, TestHost, TestHostOptions, WorkflowOutcome, WorkflowRun,
} from './testhost.ts'
