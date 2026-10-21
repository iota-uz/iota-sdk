import { test as base, expect } from '@playwright/test'
import { createEnvironmentLeaseTest } from '../src/index.js'
const events: string[] = []
const test = createEnvironmentLeaseTest(base, {
  scope: 'test', runName: 'lease-characterisation',
  lifecycle: info => ({
    start: async () => { events.push(`start:${info.environmentName}`); return { runId: info.environmentName, baseURL: 'http://lease.test', artifactDir: '/tmp/lease' } },
    ready: async () => {},
    stop: async environment => { events.push(`stop:${environment.runId}`) },
    close: async () => { events.push('driver') },
  }),
})
test.describe.configure({ mode: 'serial' })
// Characterisation: falsely green if a page uses an explicit URL instead of the fixture's baseURL.
test('test lease supplies the consumer page origin', async ({ page, environmentLease }) => {
  expect(environmentLease.runId).toContain('lease-characterisation-w')
  await page.route('http://lease.test/probe', route => route.fulfill({ body: '<h1>Lease origin</h1>', contentType: 'text/html' }))
  await page.goto('/probe')
  await expect(page.getByRole('heading')).toHaveText('Lease origin')
})
// Characterisation: falsely green if cleanup is inspected before the prior test's fixture unwinds.
test('the next attempt observes cleanup and a distinct environment', async ({ environmentLease }) => {
  expect(events[1]).toBe(events[0].replace('start:', 'stop:'))
  expect(events[2]).toBe('driver')
  expect(`start:${environmentLease.runId}`).not.toBe(events[0])
})
const workerEvents: string[] = []
const workerTest = createEnvironmentLeaseTest(base, {
  scope: 'worker', runName: 'worker-characterisation', name: info => `${info.runName}-${info.parallelIndex}`,
  lifecycle: info => ({
    start: async () => { workerEvents.push(info.environmentName); return { runId: info.environmentName, baseURL: 'http://worker.test', artifactDir: '/tmp/worker' } },
    ready: async () => {}, stop: async () => {},
  }),
})
// Characterisation: falsely green if both assertions acquire independent test-scoped leases.
workerTest('worker lease can be injected into worker fixtures', async ({ environmentLease }) => {
  expect(workerEvents).toEqual([environmentLease.runId])
})
workerTest('worker lease remains shared across tests', async ({ environmentLease }) => {
  expect(workerEvents).toEqual([environmentLease.runId])
})
