// Test support for extension packages (`@iota-uz/sdk/runtime/testing`): synthetic typed fixtures, fake
// external effects and an in-memory host that runs handlers without Granite or a database.
export { collectManifest, ManifestError } from './manifest.ts'
export type { CollectInput } from './manifest.ts'
export { EPOCH, OVERLAY_OF, fixture, overlayFixture } from './fixtures.ts'
export type { Fixture, FixtureValues } from './fixtures.ts'
export { assert, AssertionError } from './assert.ts'
export type { Assert, ErrorMatcher } from './assert.ts'
export { UnmatchedRequestError, fakeEffects } from './fakes.ts'
export type {
  EmailMessage, FakeChannel, FakeEffects, FakeHttp, FakeHttpRequest, FakeHttpResponse, InAppMessage, PushMessage, SmsMessage,
} from './fakes.ts'
export { TestHostError, createTestHost, describe, test } from './testhost.ts'
export type {
  AnyWriteRule, Delivery, DeliveryStatus, DeliveredSignal, EmittedEvent, PendingWork, SentNotice, StartedWorkflow, TestGuard, TestHost, TestHostOptions, WorkflowOutcome, WorkflowRun,
} from './testhost.ts'
