import { render } from '@solidjs/testing-library';
import { createSignal, onCleanup } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
beforeEach(()=>vi.stubGlobal('matchMedia',()=>({matches:false,addEventListener(){},removeEventListener(){}})));
afterEach(()=>vi.unstubAllGlobals());
import { BiChatLayout } from './BiChatLayout';
import { IotaContextProvider } from '../context/IotaContext';
import type { IotaContext } from '../types/iota';

// Falsely green if only the same text survives: assert owner identity and draft DOM, not restored server state.
describe('BiChatLayout route ownership',()=>{
 it('keeps the chat owner when a new conversation acquires a session URL',()=>{
  const [route,setRoute]=createSignal('/');
  let mounts=0,cleanups=0;
  const Chat=()=>{mounts++;onCleanup(()=>cleanups++);return <input data-testid="draft" />};
  const context={locale:{language:'en',translations:{}},config:{basePath:'/chat'},user:{permissions:[]}} as unknown as IotaContext;
  const view=render(()=><IotaContextProvider context={context}><BiChatLayout routeKey={route()} renderSidebar={()=><div>Sidebar</div>}><Chat /></BiChatLayout></IotaContextProvider>);
  const input=view.getByTestId('draft') as HTMLInputElement;
  input.value='unfinished draft';
  setRoute('/session/new-session');
  expect(view.getByTestId('draft')).toBe(input);
  expect(input.value).toBe('unfinished draft');
  expect(mounts).toBe(1);expect(cleanups).toBe(0);
  view.unmount();expect(cleanups).toBe(1);
 });
});
