import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * CodeOutputsPanel Component
 * Displays code interpreter outputs (images, text, errors)
 *
 * Output types:
 * - image: Base64-encoded image data (content is base64 string, mimeType specifies format)
 * - text: Plain text output from code execution
 * - error: Error messages from failed code execution
 */
import { Download } from '../icons';
import type { CodeOutput } from '../types';
import { formatFileSize } from '../utils/fileUtils';
import { useTranslation } from '../hooks/useTranslation';
interface CodeOutputsPanelProps {
    outputs: CodeOutput[];
}
function toBase64(str: string): string {
    // btoa() only supports Latin1; this converts UTF-8 bytes to a binary string first.
    const bytes = new TextEncoder().encode(str);
    let binary = '';
    for (let i = 0; i < bytes.length; i++) {
        binary += String.fromCharCode(bytes[i]);
    }
    return btoa(binary);
}
function CodeOutputsPanel(solidProps1: CodeOutputsPanelProps) {
    const solidState2 = useTranslation();
    return <Show when={!(!solidProps1.outputs || solidProps1.outputs.length === 0)}>{_visible => {
            return (<div class="mb-2 p-3 bg-gray-50 dark:bg-gray-900/50 rounded-lg border border-gray-200 dark:border-gray-700">
      <div class="text-xs font-semibold text-gray-600 dark:text-gray-400 mb-2">
        {solidState2.t('BiChat.CodeOutput.Title')}
      </div>
      <div class="space-y-2">
        {solidProps1.outputs.map((output, index) => (<div>
            {output.type === 'image' && (<div class="relative group">
                <img src={output.content.startsWith('data:')
                            ? output.content
                            : `data:${output.mimeType || 'image/png'};base64,${output.content}`} alt={output.filename || solidState2.t('BiChat.CodeOutput.CodeOutput')} class="max-w-full rounded border border-gray-300 dark:border-gray-600"/>
                {/* File info overlay */}
                {output.filename && (<div class="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/60 to-transparent p-2 opacity-0 group-hover:opacity-100 transition-opacity">
                    <div class="flex items-center justify-between text-white text-xs">
                      <span class="truncate">{output.filename}</span>
                      {output.sizeBytes && (<span class="text-gray-300">{formatFileSize(output.sizeBytes)}</span>)}
                    </div>
                  </div>)}
              </div>)}
            {output.type === 'text' && (<div>
                <pre class="text-xs bg-white dark:bg-gray-800 p-2 rounded overflow-x-auto border border-gray-200 dark:border-gray-700">
                  <code class="text-gray-900 dark:text-gray-100">{output.content}</code>
                </pre>
                {/* File download link */}
                {output.filename && (<div class="flex items-center gap-2 mt-1 text-xs">
                    <a href={`data:${output.mimeType || 'text/plain'};base64,${toBase64(output.content)}`} download={output.filename} class="flex items-center gap-1 text-blue-600 dark:text-blue-400 hover:underline">
                      <Download size={12} weight="bold"/>
                      {output.filename}
                    </a>
                    {output.sizeBytes && (<span class="text-gray-500 dark:text-gray-400">
                        ({formatFileSize(output.sizeBytes)})
                      </span>)}
                  </div>)}
              </div>)}
            {output.type === 'error' && (<div class="text-xs text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 p-2 rounded border border-red-200 dark:border-red-800">
                <div class="font-semibold mb-1">{solidState2.t('BiChat.Error.Label')}</div>
                <pre class="whitespace-pre-wrap">{output.content}</pre>
              </div>)}
          </div>))}
      </div>
    </div>);
        }}</Show>;
}
export { CodeOutputsPanel };
export default CodeOutputsPanel;
