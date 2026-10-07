const base=document.body.dataset.docsBase??'';
const input=document.querySelector<HTMLInputElement>('#docs-search');
const results=document.querySelector<HTMLElement>('#search-results');
let generation=0;
let searchModule:Promise<any>|undefined;
input?.addEventListener('input',async()=>{
 const current=++generation;const value=input.value.trim();if(!value){if(results)results.hidden=true;return}
 try{searchModule??=import(/* @vite-ignore */ base+'/_pagefind/pagefind.js');const search=await searchModule;const found=await search.search(value);const rows=await Promise.all(found.results.slice(0,8).map((row:any)=>row.data()));if(current!==generation||!results)return;results.replaceChildren();for(const row of rows){const link=document.createElement('a');link.href=row.url;link.textContent=row.meta.title??row.url;results.append(link)}if(!rows.length)results.textContent='No results';results.hidden=false}catch{if(results){results.textContent='Search is available after the static build';results.hidden=false}}
});
document.querySelector('#theme-toggle')?.addEventListener('click',()=>{const dark=document.documentElement.classList.toggle('dark');localStorage.setItem('docs-theme',dark?'dark':'light')});
const toggle=document.querySelector<HTMLButtonElement>('#nav-toggle');toggle?.addEventListener('click',()=>{const open=document.body.classList.toggle('nav-open');toggle.setAttribute('aria-expanded',String(open))});
let tabGroup=0;
for(const container of document.querySelectorAll<HTMLElement>('[data-doc-tabs]')){
 const id='docs-tabs-'+(++tabGroup);
 const panels=Array.from(container.querySelectorAll<HTMLElement>(':scope > [data-doc-tab-panel]'));
 const labels=JSON.parse(container.dataset.labels??'[]') as string[];
 const tabs=document.createElement('div');tabs.setAttribute('role','tablist');tabs.setAttribute('aria-label','Documentation examples');
 const buttons=panels.map((panel,index)=>{
 const button=document.createElement('button');button.textContent=labels[index]??String(index+1);
 button.id=id+'-tab-'+index;panel.id=id+'-panel-'+index;
 button.setAttribute('role','tab');button.setAttribute('aria-controls',panel.id);
 panel.setAttribute('role','tabpanel');panel.setAttribute('aria-labelledby',button.id);
 panel.tabIndex=0;tabs.append(button);return button;
 });
 const activate=(index:number,focus=false)=>{panels.forEach((panel,other)=>panel.hidden=other!==index);buttons.forEach((button,other)=>{button.setAttribute('aria-selected',String(other===index));button.tabIndex=other===index?0:-1});if(focus)buttons[index].focus()};
 buttons.forEach((button,index)=>{button.addEventListener('click',()=>activate(index));button.addEventListener('keydown',event=>{
 let target:number|undefined;if(event.key==='ArrowRight')target=(index+1)%buttons.length;if(event.key==='ArrowLeft')target=(index-1+buttons.length)%buttons.length;if(event.key==='Home')target=0;if(event.key==='End')target=buttons.length-1;
 if(target!==undefined){event.preventDefault();activate(target,true)}
 })});activate(0);container.prepend(tabs);
}
const diagrams=document.querySelectorAll('.mermaid');if(diagrams.length){void import('mermaid').then(({default:mermaid})=>{mermaid.initialize({startOnLoad:false,theme:document.documentElement.classList.contains('dark')?'dark':'default',securityLevel:'strict'});return mermaid.run({nodes:diagrams as NodeListOf<HTMLElement>})})}

document.addEventListener('keydown',event=>{if(event.key==='Escape'){if(document.body.classList.contains('nav-open')){document.body.classList.remove('nav-open');toggle?.setAttribute('aria-expanded','false');toggle?.focus()}if(results)results.hidden=true}});
