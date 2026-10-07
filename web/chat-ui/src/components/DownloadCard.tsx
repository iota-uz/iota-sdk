import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * DownloadCard component
 * File-download card for artifacts (Excel, PDF)
 * with type-specific icon, metadata, and download action.
 */
import { DownloadSimple } from '../icons';
import type { Artifact } from '../types';
import { getFileVisual } from '../utils/fileUtils';
interface DownloadCardProps {
    artifact: Artifact;
}
const MIME_BY_TYPE: Record<string, string> = {
    excel: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    pdf: 'application/pdf',
};
export function DownloadCard(solidProps1: DownloadCardProps) {
    const artifact = () => solidProps1.artifact;
    const visual = getFileVisual(MIME_BY_TYPE[artifact().type], artifact().filename);
    const Icon = visual.icon;
    return (<a href={artifact().url} download={artifact().filename} class={[
            'group/dl flex items-center gap-2.5 rounded-xl',
            'border border-gray-200/80 dark:border-gray-700/60',
            'bg-white dark:bg-gray-800/60',
            'px-2.5 py-2',
            'transition-all duration-150',
            'hover:border-gray-300 dark:hover:border-gray-600',
            'hover:shadow-sm',
            'hover:-translate-y-px active:translate-y-0',
        ].join(' ')}>
      {/* File type icon */}
      <div class={`flex-shrink-0 flex items-center justify-center w-10 h-10 rounded-lg ${visual.bgColor}`}>
        <Icon size={20} weight="duotone" className={visual.iconColor}/>
      </div>

      {/* Content */}
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-1.5 min-w-0">
          <span class="text-[13px] font-medium text-gray-900 dark:text-gray-100 truncate">
            {artifact().filename}
          </span>
          <span class="flex-shrink-0 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider rounded bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400">
            {visual.label}
          </span>
        </div>
        {(artifact().sizeReadable || artifact().rowCount !== undefined) && (<div class="flex items-center gap-1.5 text-[11px] text-gray-400 dark:text-gray-500">
            {artifact().sizeReadable && <span>{artifact().sizeReadable}</span>}
            {artifact().sizeReadable && artifact().rowCount !== undefined && (<span class="w-0.5 h-0.5 rounded-full bg-gray-300 dark:bg-gray-600"/>)}
            {artifact().rowCount !== undefined && (<span>{artifact().rowCount?.toLocaleString()} rows</span>)}
          </div>)}
        {artifact().description && (<p class="mt-0.5 text-[11px] text-gray-400 dark:text-gray-500 line-clamp-1">
            {artifact().description}
          </p>)}
      </div>

      {/* Download arrow */}
      <div class="flex-shrink-0 p-1 text-gray-400 dark:text-gray-500 group-hover/dl:text-gray-600 dark:group-hover/dl:text-gray-300 transition-colors duration-150">
        <DownloadSimple size={16} weight="bold" className="transition-transform duration-200 group-hover/dl:translate-y-0.5"/>
      </div>
    </a>);
}
