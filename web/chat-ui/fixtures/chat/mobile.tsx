import { createSignal } from 'solid-js';
import { mountSolidClientRoute, CLIENT_BOOTSTRAP_VERSION, SDK_IDENTITY } from '@iota-uz/sdk/solid';
import { BiChatLayout, ChatHeader, Sidebar, ChatSession, IotaContextProvider, type ChatDataSource, type IotaContext } from '@iota-uz/sdk/chat-ui';
import './fixture.css';

const session = {id:'fixture',title:'A long existing conversation title',status:'active' as const,pinned:false,createdAt:new Date().toISOString(),updatedAt:new Date().toISOString()};
const source = {
 listSessions: async()=>({sessions:[session],hasMore:false}),
 fetchSession: async()=>({session,turns:[],pendingQuestion:null}),
 fetchSessionArtifacts: async()=>({artifacts:[],hasMore:false}),
} as unknown as ChatDataSource;
const translations = {
 'BiChat.Layout.OpenSidebar':'Open sidebar',
 'BiChat.Sidebar.CloseSidebar':'Close sidebar',
 'BiChat.Sidebar.CollapseSidebar':'Collapse sidebar',
 'BiChat.Sidebar.ExpandSidebar':'Expand sidebar',
 'BiChat.Chat.NewChat':'New chat',
 'BiChat.Artifacts.Title':'Artifacts',
 'BiChat.Common.Close':'Close',
};
const context = {locale:{language:'en',translations},config:{basePath:'/chat'},user:{permissions:[],firstName:'Fixture',lastName:'User'},tenant:{id:'fixture',name:'Fixture'}} as unknown as IotaContext;
function Fixture() {
 const [existing,setExisting] = createSignal(false);
 return <IotaContextProvider context={context}>
  <BiChatLayout renderSidebar={props=><Sidebar {...props} dataSource={source} onNewChat={()=>setExisting(false)} onSessionSelect={()=>setExisting(true)}/>}>
   <div class="flex min-w-0 flex-1 flex-col">
    {existing() ? <ChatSession dataSource={source} sessionId="fixture" showArtifactsPanel/> : <><ChatHeader session={null}/><button onClick={()=>setExisting(true)}>Open existing</button></>}
   </div>
  </BiChatLayout>
 </IotaContextProvider>;
}
mountSolidClientRoute({
 root:document.getElementById('app')!,portals:document.getElementById('portals')!,component:Fixture,props:{},
 context:{bootstrapVersion:CLIENT_BOOTSTRAP_VERSION,protocolVersion:SDK_IDENTITY.protocolVersion,sdkReleaseVersion:SDK_IDENTITY.releaseVersion,sdkCommit:SDK_IDENTITY.sourceCommit,initial:{},theme:'light'},
});
