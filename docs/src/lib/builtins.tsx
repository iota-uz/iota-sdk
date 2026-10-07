import type { JSX } from 'solid-js';
type Children={children?:JSX.Element};
export function Callout(props:Children&{type?:string;emoji?:string}){return <aside class="docs-callout" data-kind={props.type??'info'}>{props.emoji&&<span>{props.emoji}</span>}{props.children}</aside>}
export function Steps(props:Children){return <div class="docs-steps">{props.children}</div>}
const Tab=(props:Children)=> <section data-doc-tab-panel>{props.children}</section>;
export const Tabs=Object.assign((props:Children&{items?:string[]})=><div data-doc-tabs data-labels={JSON.stringify(props.items??[])}>{props.children}</div>,{Tab});
const Card=(props:Children&{title:string;href:string})=><a class="docs-card" href={props.href.startsWith('/')&&!props.href.startsWith('//')&&!props.href.startsWith(import.meta.env.BASE_URL.replace(/\/$/,''))?import.meta.env.BASE_URL.replace(/\/$/,'')+props.href:props.href}><strong>{props.title}</strong>{props.children}</a>;
export const Cards=Object.assign((props:Children)=><div class="docs-cards">{props.children}</div>,{Card});
