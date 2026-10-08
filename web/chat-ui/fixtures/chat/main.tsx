import {render} from 'solid-js/web';
import {createSignal,Show} from 'solid-js';
import {ChatSession,IotaContextProvider,type ChatDataSource,type IotaContext} from '@iota-uz/sdk/chat-ui';
import './fixture.css';
const stats={aborted:0,stopped:0,attempts:0};Object.assign(window,{chatFixture:stats});
const makeSession=(id:string)=>({id,title:'Fixture conversation',status:'active' as const,pinned:false,createdAt:new Date().toISOString(),updatedAt:new Date().toISOString()});
let content='';let mode='success';
const source={
 createSession:async()=>makeSession('fixture'),
 fetchSession:async(id:string)=>({session:makeSession(id),turns:content?[{id:'turn',sessionId:id,createdAt:new Date().toISOString(),userTurn:{id:'user',content:'hello',attachments:[],createdAt:new Date().toISOString()},assistantTurn:{id:'answer',role:'assistant',content,citations:[],artifacts:[],codeOutputs:[],lifecycle:'complete',createdAt:new Date().toISOString()}}]:[],pendingQuestion:null}),
 async *sendMessage(_id:string,_message:string,_files:any,signal?:AbortSignal){
 stats.attempts++;
 if(mode==='error'&&stats.attempts===1){yield {type:'error',error:'fixture unavailable',errorSource:'transport'};return}
 yield {type:'chunk',content:'Browser streamed response'};
 if(mode==='wait'){await new Promise<void>(resolve=>{signal?.addEventListener('abort',()=>{stats.aborted++;resolve()},{once:true})});yield {type:'chunk',content:'LATE DETACHED CONTENT'};return}
 content='Browser streamed response with **markdown**';yield {type:'done'};
 },
 stopGeneration:async()=>{stats.stopped++},
 getStreamStatus:async()=>({active:false}),
 listSessions:async()=>({sessions:[],hasMore:false}),
} as unknown as ChatDataSource;
function Fixture(){
 const [mounted,setMounted]=createSignal(true);
 const context={locale:{language:'en',translations:{'BiChat.Input.Placeholder':'Write a message','BiChat.Input.MessageInput':'Message','BiChat.Input.SendMessage':'Send','BiChat.Common.Cancel':'Stop','BiChat.Common.Retry':'Retry'}},config:{basePath:'/chat'},user:{permissions:[],firstName:'Fixture',lastName:'User'},tenant:{id:'fixture',name:'Fixture'}} as unknown as IotaContext;
 return <IotaContextProvider context={context}><div style={{height:'100dvh',display:'flex','flex-direction':'column'}}>
 <div><button onClick={()=>{mode='wait'}}>Wait mode</button><button onClick={()=>{mode='error';stats.attempts=0}}>Error mode</button><button onClick={()=>setMounted(false)}>Leave chat</button></div>
 <Show when={mounted()} fallback={<p>Left conversation</p>}><ChatSession dataSource={source} sessionId="fixture"/></Show>
 </div></IotaContextProvider>
}
render(()=> <Fixture/>,document.getElementById('app')!);
