import { createMemo, Index, Show, type JSX } from 'solid-js';
import { Dynamic } from 'solid-js/web';
import { unified } from 'unified';
import remarkParse from 'remark-parse';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import remarkRehype from 'remark-rehype';
import rehypeKatex from 'rehype-katex';
import rehypeSanitize from 'rehype-sanitize';
import rehypeStringify from 'rehype-stringify';
import { processCitations } from '../utils/citationProcessor';
import { parseChartDataFromJsonString } from '../utils/chartSpec';
import { normalizeLatexDelimiters } from '../utils/markdownMath';
import type { Citation } from '../types';
import { TableWithExport } from './TableWithExport';
import { ChartCard } from './ChartCard';
import { CodeBlock } from './CodeBlock';
interface MarkdownRendererProps {
    content: string;
    citations?: Citation[] | null;
    sendMessage?: (content: string) => void;
    sendDisabled?: boolean;
    copyLabel?: string;
    copiedLabel?: string;
    exportLabel?: string;
}
interface AstNode {
    type: string;
    tagName?: string;
    value?: string;
    children?: AstNode[];
    properties?: Record<string, unknown>;
}
const parser = unified().use(remarkParse).use(remarkGfm).use(remarkMath).use(remarkRehype).use(rehypeSanitize).use(rehypeKatex, { strict: 'ignore', trust: false, output: 'mathml' });
const serializer = unified().use(rehypeStringify);
function text(node: AstNode): string { return node.value ?? node.children?.map(text).join('') ?? ''; }
function attributes(properties: Record<string, unknown> = {}) {
    return Object.fromEntries(Object.entries(properties).map(([key, value]) => [
        key === 'className' ? 'class' : key.startsWith('aria') || key.startsWith('data') ? key.replace(/[A-Z]/g, letter => '-' + letter.toLowerCase()) : key,
        Array.isArray(value) ? value.join(' ') : value,
    ]));
}
function Nodes(props: {
    nodes: AstNode[];
    options: MarkdownRendererProps;
}): JSX.Element {
    return <Index each={props.nodes}>{node => <Node node={node()} options={props.options}/>}</Index>;
}
function Code(props: {
    node: AstNode;
    options: MarkdownRendererProps;
}) {
    const child = createMemo(() => props.node.children?.find(node => node.tagName === 'code') ?? props.node);
    const classes = createMemo(() => String((child().properties?.className as string[] | undefined)?.join(' ') ?? ''));
    const language = createMemo(() => /language-([\w+-]+)/.exec(classes())?.[1] ?? 'text');
    const value = createMemo(() => text(child()).replace(/\n$/, ''));
    const chart = createMemo(() => language() === 'chart' || (language() === 'json' && value().includes('"chartType"')) ? parseChartDataFromJsonString(value(), 'Chart') : null);
    return <Show when={chart()} keyed fallback={<CodeBlock language={language()} value={value()} copyLabel={props.options.copyLabel} copiedLabel={props.options.copiedLabel}/>}>{data => <ChartCard chartData={data}/>}</Show>;
}
function Node(props: {
    node: AstNode;
    options: MarkdownRendererProps;
}): JSX.Element {
    const tag = () => props.node.tagName ?? 'span';
    const nested = () => <Nodes nodes={props.node.children ?? []} options={props.options}/>;
    return <Show when={props.node.type !== 'text'} fallback={props.node.value}>
  <Show when={tag() !== 'pre'} fallback={<Code node={props.node} options={props.options}/>}>
   <Show when={tag() !== 'table'} fallback={<TableWithExport sendMessage={props.options.sendMessage} disabled={props.options.sendDisabled} exportLabel={props.options.exportLabel}>{nested()}</TableWithExport>}>
    <Show when={tag() !== 'math'} fallback={<span innerHTML={String(serializer.stringify({ type: 'root', children: [props.node] } as never))}/>}>
     <Dynamic component={tag()} {...attributes(props.node.properties)} class={['markdown-' + tag(), attributes(props.node.properties).class].filter(Boolean).join(' ')} target={tag() === 'a' ? '_blank' : undefined} rel={tag() === 'a' ? 'noopener noreferrer' : undefined}>
      {nested()}
     </Dynamic>
    </Show>
   </Show>
  </Show>
 </Show>;
}
export function MarkdownRenderer(props: MarkdownRendererProps) {
    const tree = createMemo(() => {
        const content = normalizeLatexDelimiters(processCitations(props.content, props.citations).content);
        return parser.runSync(parser.parse(content)) as unknown as AstNode;
    });
    return <div class="markdown-content"><Nodes nodes={tree().children ?? []} options={props}/></div>;
}
export default MarkdownRenderer;
