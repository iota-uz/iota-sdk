import { render } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import { IotaContextProvider } from '../context/IotaContext';
import type { IotaContext } from '../types/iota';
import type { ConversationTurn } from '../types';
import { StreamError } from './StreamError';
import { UserAvatar } from './UserAvatar';
import { UserTurnView } from './UserTurnView';

vi.mock('../context/ChatContext', () => ({ useChatMessaging: () => ({ handleCopy: () => {}, handleEdit: () => {} }) }));

const context = (translations: Record<string, string> = {}, user = { id: 1, email: 'a@b', permissions: [], firstName: 'Малик', lastName: 'Фаттаев' }) =>
 ({ locale: { language: 'ru', translations }, config: { basePath: '/chat' }, user }) as unknown as IotaContext;
const turn = (author?: { id: string; firstName: string; lastName: string; initials: string }) => ({
 id: 't', sessionId: 's', createdAt: new Date().toISOString(),
 userTurn: { id: 'u', content: 'hi', attachments: [], createdAt: new Date().toISOString(), author },
}) as unknown as ConversationTurn;

describe('StreamError', () => {
 const translations = { 'BiChat.StreamError.Failed.Title': 'Не удалось получить ответ', 'BiChat.StreamError.Failed.Details': 'Повторите попытку.' };
 it.each(['stream_failed', 'An error occurred while processing your request'])('translates the generic server failure %j', (error) => {
  const view = render(() => <IotaContextProvider context={context(translations)}><StreamError error={error}/></IotaContextProvider>);
  expect(view.getByText('Не удалось получить ответ')).toBeTruthy();
  expect(view.queryByText(error)).toBeNull();
 });
});

describe('avatar initials', () => {
 it('uses the viewer when a turn has no author', () => {
  const view = render(() => <IotaContextProvider context={context()}><UserTurnView turn={turn()}/></IotaContextProvider>);
  expect(view.getByText('МФ')).toBeTruthy();
  expect(view.queryByText('U')).toBeNull();
 });
 it('keeps the real author of a shared turn', () => {
  const view = render(() => <IotaContextProvider context={context()}><UserTurnView turn={turn({ id: '2', firstName: 'Ada', lastName: 'Lovelace', initials: 'AL' })}/></IotaContextProvider>);
  expect(view.getByText('AL')).toBeTruthy();
 });
 it('shows a neutral placeholder rather than U when nothing is known', () => {
  const view = render(() => <UserAvatar firstName="" lastName=""/>);
  expect(view.getByText('?')).toBeTruthy();
  expect(view.queryByText('U')).toBeNull();
 });
});
