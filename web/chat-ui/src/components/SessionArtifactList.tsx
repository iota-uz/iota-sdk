import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { ChartBar, Code, FileCsv, Image as ImageIcon, Package, Table as TableIcon, } from '../icons';
import type { SessionArtifact } from '../types';
import { useTranslation } from '../hooks/useTranslation';
import { formatFileSize, getFileVisual, CHART_VISUAL, type FileVisual } from '../utils/fileUtils';
import { getArtifactName, isImageArtifact } from '../utils/artifactHelpers';
interface SessionArtifactListProps {
    artifacts: SessionArtifact[];
    selectedArtifactId?: string;
    onSelect: (artifact: SessionArtifact) => void;
}
const TYPE_LABEL_KEYS: Record<string, string> = {
    chart: 'BiChat.Artifacts.GroupCharts',
    table: 'BiChat.Artifacts.GroupTables',
    code_output: 'BiChat.Artifacts.GroupCodeOutputs',
    export: 'BiChat.Artifacts.GroupExports',
    attachment: 'BiChat.Artifacts.GroupAttachments',
    other: 'BiChat.Artifacts.GroupOther',
};
function getGroupIcon(type: string): JSX.Element {
    const cls = 'h-3.5 w-3.5';
    switch (type) {
        case 'chart':
            return <ChartBar className={cls} weight="bold"/>;
        case 'table':
            return <TableIcon className={cls} weight="bold"/>;
        case 'code_output':
            return <Code className={cls} weight="bold"/>;
        case 'export':
            return <FileCsv className={cls} weight="bold"/>;
        case 'attachment':
            return <ImageIcon className={cls} weight="bold"/>;
        default:
            return <Package className={cls} weight="bold"/>;
    }
}
function ImageThumbnail(solidProps1: {
    src: string;
    alt: string;
}) {
    const [failed, setFailed] = createSignal(false);
    return <>{createMemo(() => {
            if (failed()) {
                return (<div class="w-full aspect-video rounded-lg bg-violet-50 dark:bg-violet-900/30 flex items-center justify-center">
        <ImageIcon className="h-6 w-6 text-violet-400 dark:text-violet-500" weight="duotone"/>
      </div>);
            }
            return (<img src={solidProps1.src} alt={solidProps1.alt} onError={() => setFailed(true)} class="w-full rounded-lg object-cover max-h-32 bg-gray-100 dark:bg-gray-800"/>);
        })}</>;
}
function getArtifactFileVisual(artifact: SessionArtifact): FileVisual {
    if (artifact.type === 'chart') {
        return CHART_VISUAL;
    }
    if (artifact.type === 'code_output') {
        const v = getFileVisual(artifact.mimeType, getArtifactName(artifact));
        // Code outputs get a sky accent unless they resolve to something specific (image, etc.)
        if (v.label === 'TEXT' || v.label === 'FILE') {
            return { ...v, iconColor: 'text-sky-600 dark:text-sky-400', bgColor: 'bg-sky-100 dark:bg-sky-900/40' };
        }
        return v;
    }
    return getFileVisual(artifact.mimeType, getArtifactName(artifact));
}
function groupArtifactsByType(artifacts: SessionArtifact[]): Array<{
    type: string;
    items: SessionArtifact[];
}> {
    const grouped = new Map<string, SessionArtifact[]>();
    for (const artifact of artifacts) {
        const type = artifact.type || 'other';
        const existing = grouped.get(type);
        if (existing) {
            existing.push(artifact);
            continue;
        }
        grouped.set(type, [artifact]);
    }
    return Array.from(grouped.entries())
        .map(([type, items]) => ({
        type,
        items: items.sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt)),
    }))
        .sort((a, b) => a.type.localeCompare(b.type));
}
export function SessionArtifactList(solidProps2: SessionArtifactListProps) {
    const solidState3 = useTranslation();
    const grouped = createMemo(() => groupArtifactsByType(solidProps2.artifacts));
    return <>{createMemo(() => {
            if (solidProps2.artifacts.length === 0) {
                return (<div class="flex h-full flex-col items-center justify-center gap-3 px-4 py-12 text-center">
        <div class="flex h-12 w-12 items-center justify-center rounded-xl bg-gray-100 dark:bg-gray-800">
          <Package className="h-6 w-6 text-gray-400 dark:text-gray-500" weight="duotone"/>
        </div>
        <div>
          <p class="text-sm font-medium text-gray-500 dark:text-gray-400">
            {solidState3.t('BiChat.Artifacts.Empty')}
          </p>
          <p class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">
            {solidState3.t('BiChat.Artifacts.EmptySubtitle')}
          </p>
          <p class="mt-2 text-xs text-gray-400 dark:text-gray-500">
            {solidState3.t('BiChat.Artifacts.EmptyHint')}
          </p>
        </div>
      </div>);
            }
            return (<div class="space-y-5">
      {grouped().map((group) => (<section>
          <div class="mb-2 flex items-center gap-1.5 px-0.5">
            <span class="text-gray-400 dark:text-gray-500">{getGroupIcon(group.type)}</span>
            <h3 class="text-[11px] font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
              {TYPE_LABEL_KEYS[group.type] ? solidState3.t(TYPE_LABEL_KEYS[group.type]) : group.type.replace(/_/g, ' ')}
            </h3>
            <span class="ml-auto text-[10px] tabular-nums text-gray-400 dark:text-gray-500">
              {group.items.length}
            </span>
          </div>
          <div class="space-y-1">
            {group.items.map((artifact) => {
                        const isSelected = artifact.id === solidProps2.selectedArtifactId;
                        const visual = getArtifactFileVisual(artifact);
                        const Icon = visual.icon;
                        const artifactName = getArtifactName(artifact);
                        return (<button type="button" onClick={() => solidProps2.onSelect?.(artifact)} class={`cursor-pointer group/item w-full rounded-lg border px-3 py-2 text-left transition-all duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 ${isSelected
                                ? 'border-primary-200 bg-primary-50/80 shadow-sm dark:border-primary-800/60 dark:bg-primary-950/40'
                                : 'border-transparent bg-white hover:border-gray-200 hover:bg-gray-50 hover:shadow-sm dark:bg-gray-900 dark:hover:border-gray-700/80 dark:hover:bg-gray-800/60'}`}>
                  {isImageArtifact(artifact) && artifact.url ? (<div>
                      <ImageThumbnail src={artifact.url} alt={artifactName}/>
                      <div class="mt-2">
                        <span class="block truncate text-[13px] font-medium text-gray-900 dark:text-gray-100">
                          {artifactName}
                        </span>
                        <span class="flex items-center gap-1.5 text-[11px] text-gray-400 dark:text-gray-500">
                          <span>{formatFileSize(artifact.sizeBytes)}</span>
                          {artifact.description && (<>
                              <span class="w-0.5 h-0.5 rounded-full bg-gray-300 dark:bg-gray-600"/>
                              <span class="truncate">{artifact.description}</span>
                            </>)}
                        </span>
                      </div>
                    </div>) : (<div class="flex items-center gap-2.5">
                      <span class={`flex-shrink-0 flex items-center justify-center w-10 h-10 rounded-lg ${visual.bgColor} ${visual.iconColor}`}>
                        <Icon size={20} weight="duotone"/>
                      </span>
                      <span class="min-w-0 flex-1">
                        <span class="block truncate text-[13px] font-medium text-gray-900 dark:text-gray-100">
                          {artifactName}
                        </span>
                        <span class="flex items-center gap-1.5 text-[11px] text-gray-400 dark:text-gray-500">
                          <span>{formatFileSize(artifact.sizeBytes)}</span>
                          {artifact.description && (<>
                              <span class="w-0.5 h-0.5 rounded-full bg-gray-300 dark:bg-gray-600"/>
                              <span class="truncate">{artifact.description}</span>
                            </>)}
                        </span>
                      </span>
                    </div>)}
                </button>);
                    })}
          </div>
        </section>))}
    </div>);
        })}</>;
}
