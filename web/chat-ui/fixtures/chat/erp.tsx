import { mountSolidClientRoute, CLIENT_BOOTSTRAP_VERSION, SDK_IDENTITY } from '@iota-uz/sdk/solid';
import { BiChatLayout, Sidebar, ChatSession, IotaContextProvider, type ChatDataSource, type IotaContext } from '@iota-uz/sdk/chat-ui';
import ru from '../../../../modules/bichat/presentation/locales/ru.json';
import './fixture.css';

const flatten = (value: unknown, prefix: string, out: Record<string, string>): Record<string, string> => {
 if (typeof value === 'string') out[prefix] = value;
 else if (value && typeof value === 'object') for (const [key, child] of Object.entries(value)) flatten(child, prefix ? `${prefix}.${key}` : key, out);
 return out;
};
const translations = flatten(ru, '', {});
const params = new URLSearchParams(location.search);
const hasSession = params.get('session') === '1';
const nameless = params.get('nameless') === '1';
const now = new Date().toISOString();
const owner = nameless ? undefined : { id: '1', firstName: 'Малик', lastName: 'Фаттаев', initials: 'МФ' };
const session = { id: 's1', title: 'Выручка по филиалам', status: 'active' as const, pinned: false, createdAt: now, updatedAt: now, owner };
const turn = {
 id: 't1', sessionId: 's1', createdAt: now,
 userTurn: { id: 'u1', content: 'Покажи выручку за месяц', attachments: [], createdAt: now, author: nameless ? undefined : owner },
 assistantTurn: undefined,
};
const stats = { sends: 0 };
Object.assign(window, { erpFixture: stats });
const source = {
 listSessions: async () => ({ sessions: [session], hasMore: false }),
 fetchSession: async (id: string) => ({ session: { ...session, id }, turns: [turn], pendingQuestion: null }),
 fetchSessionArtifacts: async () => ({ artifacts: [], hasMore: false }),
 createSession: async () => ({ ...session, id: 's2' }),
 getStreamStatus: async () => ({ active: false }),
 async *sendMessage() { stats.sends++; yield { type: 'error', error: 'stream_failed' }; },
} as unknown as ChatDataSource;
const context = { locale: { language: 'ru', translations }, config: { basePath: '/chat' }, user: { id: 1, email: 'm@x', permissions: [], firstName: 'Малик', lastName: 'Фаттаев' }, tenant: { id: 't', name: 'T' } } as unknown as IotaContext;
function Fixture() {
 return <IotaContextProvider context={context}>
  <BiChatLayout renderSidebar={props => <Sidebar {...props} dataSource={source}/>}>
   <ChatSession dataSource={source} sessionId={hasSession ? 's1' : undefined} showArtifactsPanel/>
  </BiChatLayout>
 </IotaContextProvider>;
}
mountSolidClientRoute({
 root: document.getElementById('app')!, portals: document.getElementById('portals')!, component: Fixture, props: {},
 context: { bootstrapVersion: CLIENT_BOOTSTRAP_VERSION, protocolVersion: SDK_IDENTITY.protocolVersion, sdkReleaseVersion: SDK_IDENTITY.releaseVersion, sdkCommit: SDK_IDENTITY.sourceCommit, initial: {}, theme: 'light' },
});
