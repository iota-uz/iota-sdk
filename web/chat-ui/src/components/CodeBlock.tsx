import { createMemo, createSignal, onCleanup, Show } from 'solid-js';
import Prism from 'prismjs';
import 'prismjs/components/prism-javascript';
import 'prismjs/components/prism-typescript';
import 'prismjs/components/prism-jsx';
import 'prismjs/components/prism-tsx';
import 'prismjs/components/prism-python';
import 'prismjs/components/prism-go';
import 'prismjs/components/prism-sql';
import 'prismjs/components/prism-bash';
import 'prismjs/components/prism-json';
import 'prismjs/components/prism-yaml';
import 'prismjs/components/prism-java';
import 'prismjs/components/prism-c';
import 'prismjs/components/prism-cpp';
import 'prismjs/components/prism-csharp';
import 'prismjs/components/prism-ruby';
import 'prismjs/components/prism-markup-templating';
import 'prismjs/components/prism-php';
import { Copy, Check } from '../icons';
import { useTranslation } from '../hooks/useTranslation';
interface CodeBlockProps {
    language: string;
    value: string;
    inline?: boolean;
    copyLabel?: string;
    copiedLabel?: string;
}
const aliases: Record<string, string> = { js: 'javascript', ts: 'typescript', py: 'python', rb: 'ruby', yml: 'yaml', sh: 'bash', xml: 'markup', html: 'markup' };
export function CodeBlock(props: CodeBlockProps) {
    const { t } = useTranslation();
    const [copied, setCopied] = createSignal(false);
    const [failed, setFailed] = createSignal(false);
    let timer: ReturnType<typeof setTimeout> | undefined;
    let disposed = false;
    onCleanup(() => { disposed = true; clearTimeout(timer); });
    const language = createMemo(() => aliases[props.language.toLowerCase()] ?? props.language.toLowerCase() ?? 'text');
    const html = createMemo(() => {
        const grammar = Prism.languages[language()];
        return grammar ? Prism.highlight(props.value, grammar, language()) : String(Prism.util.encode(props.value));
    });
    const copy = async () => {
        try {
            await navigator.clipboard.writeText(props.value);
            if (disposed)
                return;
            setCopied(true);
            setFailed(false);
        }
        catch {
            if (disposed)
                return;
            setFailed(true);
            setCopied(false);
        }
        clearTimeout(timer);
        timer = setTimeout(() => { setCopied(false); setFailed(false); }, 2000);
    };
    return <Show when={!props.inline} fallback={<code class="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-sm dark:bg-gray-800">{props.value}</code>}>
  <div class="group relative my-4 overflow-hidden rounded-lg border border-gray-300 dark:border-gray-700">
   <div class="flex items-center justify-between border-b border-gray-300 bg-gray-100 px-4 py-2 dark:border-gray-700 dark:bg-gray-800">
    <span class="text-xs font-medium uppercase text-gray-600 dark:text-gray-400">{language()}</span>
    <button type="button" onClick={() => void copy()} class="flex items-center gap-1.5 text-xs" aria-live="polite" title={props.copyLabel ?? t('BiChat.Message.Copy')}>
     <Show when={copied()} fallback={<Copy size={16}/>}><Check size={16}/></Show>
     {failed() ? t('BiChat.Message.CopyFailed') : copied() ? props.copiedLabel ?? t('BiChat.Message.Copied') : props.copyLabel ?? t('BiChat.Message.Copy')}
    </button>
   </div>
   <pre class="bichat-code overflow-x-auto p-4 text-sm leading-6"><code class={'language-' + language()} innerHTML={html()}/></pre>
  </div>
 </Show>;
}
export default CodeBlock;
