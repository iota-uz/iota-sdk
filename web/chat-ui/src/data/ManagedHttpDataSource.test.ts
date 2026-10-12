// @vitest-environment jsdom
import { webcrypto } from 'node:crypto';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { SessionService } from '@iota-uz/sdk/client-host';
const stream = vi.hoisted(() => ({ close: vi.fn(), deliver: undefined as undefined | ((event: unknown) => void) }));
vi.mock('@iota-uz/sdk/client-host', () => ({ subscribeManagedStream: (_contract: unknown, deliver: (event: unknown) => void) => { stream.deliver = deliver; return { close: stream.close }; } }));
import { ManagedHttpDataSource } from './ManagedHttpDataSource';
vi.stubGlobal('crypto', webcrypto);
afterEach(() => stream.close.mockClear());
const session = { snapshot: () => ({ csrf: 'csrf', headers: {} }) } as unknown as SessionService;
describe('managed chat source ownership', () => {
 it('aborts pending RPC requests on disposal', async () => {
  // False green: checking a controller field would not prove that fetch receives its signal.
  let signal: AbortSignal | undefined;
  const fetcher = vi.fn((_input: RequestInfo | URL, init?: RequestInit) => new Promise<Response>((_resolve, reject) => {
   signal = init?.signal ?? undefined;
   signal?.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')), { once: true });
  }));
  const source = new ManagedHttpDataSource({ baseUrl: '', rpcEndpoint: '/rpc', fetcher }, session);
  const result = source.fetchSession('session').catch(error => error);
  expect(fetcher).toHaveBeenCalledTimes(2);
  source.dispose();
  expect(signal?.aborted).toBe(true);
  expect(await result).toBeInstanceOf(Error);
 });
 it('settles a waiting stream iterator when the owner is disposed', async () => {
  // False green: checking close alone would miss an iterator left waiting forever.
  const source = new ManagedHttpDataSource({ baseUrl: '', rpcEndpoint: '/rpc' }, session);
  const iterator = source.sendMessage('session', 'hello');
  const next = iterator.next();
  await Promise.resolve(); await Promise.resolve();
  source.dispose();
  expect((await next).value?.type).toBe('error');
  expect((await iterator.next()).done).toBe(true);
 });
 it('settles resume and ignores late events after disposal', async () => {
  // False green: no late delivery would hide updates into a detached chat owner.
  const source = new ManagedHttpDataSource({ baseUrl: '', rpcEndpoint: '/rpc' }, session);
  const onChunk = vi.fn();
  const resumed = source.resumeStream('session', 'run', onChunk);
  source.dispose();
  await resumed;
  stream.deliver?.({ type: 'chunk', content: 'late' });
  expect(onChunk).not.toHaveBeenCalled();
 });
});
