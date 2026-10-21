import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { ArrowSquareOut, DownloadSimple, FileText, SpinnerGap, WarningCircle } from '../icons';
import type { SessionArtifact } from '../types';
import { parseChartDataFromSpec, isRecord } from '../utils/chartSpec';
import { parseRenderTableDataFromMetadata } from '../utils/tableSpec';
import { ChartCard } from './ChartCard';
import { InteractiveTableCard } from './InteractiveTableCard';
import { useTranslation } from '../hooks/useTranslation';
import { getArtifactName, isImageArtifact, isPDFArtifact, isOfficeDocumentArtifact, isTextArtifact, } from '../utils/artifactHelpers';
interface SessionArtifactPreviewProps {
    artifact: SessionArtifact;
}
const TEXT_PREVIEW_MAX_CHARS = 24000;
function parseChartDataFromArtifact(artifact: SessionArtifact) {
    const metadata = artifact.metadata;
    if (!metadata || !isRecord(metadata)) {
        return null;
    }
    const spec = isRecord(metadata.spec) ? metadata.spec : metadata;
    if (!isRecord(spec)) {
        return null;
    }
    return parseChartDataFromSpec(spec, getArtifactName(artifact));
}
function isAbsoluteHTTPURL(url: string): boolean {
    return /^https?:\/\//i.test(url);
}
function WarningBox(solidProps1: {
    message: string;
}) {
    return (<div class="flex items-start gap-2.5 rounded-xl border border-amber-200/80 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800/40 dark:bg-amber-950/20 dark:text-amber-200">
      <WarningCircle className="mt-0.5 h-4 w-4 shrink-0" weight="duotone"/>
      <span class="leading-relaxed">{solidProps1.message}</span>
    </div>);
}
function ArtifactActions(solidProps2: {
    url: string;
}) {
    const solidState3 = useTranslation();
    const openLabel = createMemo(() => solidState3.t('BiChat.Artifacts.OpenInNewTab'));
    const downloadLabel = createMemo(() => solidState3.t('BiChat.Artifacts.Download'));
    return (<div class="flex items-center gap-2">
      <a href={solidProps2.url} target="_blank" rel="noreferrer" class="inline-flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50 dark:border-gray-700 dark:text-gray-200 dark:hover:bg-gray-800">
        <ArrowSquareOut className="h-3.5 w-3.5" weight="bold"/>
        {openLabel()}
      </a>
      <a href={solidProps2.url} target="_blank" rel="noreferrer" download="" class="inline-flex items-center gap-2 rounded-lg bg-primary-600 px-3 py-1.5 text-xs font-medium text-white shadow-sm transition-colors hover:bg-primary-700">
        <DownloadSimple className="h-3.5 w-3.5" weight="bold"/>
        {downloadLabel()}
      </a>
    </div>);
}
function TextArtifactPreview(solidProps4: {
    artifact: SessionArtifact;
}) {
    const solidState5 = useTranslation();
    const [loading, setLoading] = createSignal(true);
    const [error, setError] = createSignal<string | null>(null);
    const [content, setContent] = createSignal('');
    const [truncated, setTruncated] = createSignal(false);
    createEffect(on(() => [solidProps4.artifact.url, solidState5.t], () => {
        const cleanup = untrack(() => {
            if (!solidProps4.artifact.url) {
                setLoading(false);
                setError(solidState5.t('BiChat.Artifacts.TextPreviewFailed'));
                return;
            }
            const controller = new AbortController();
            setLoading(true);
            setError(null);
            setContent('');
            setTruncated(false);
            fetch(solidProps4.artifact.url, { signal: controller.signal, credentials: 'include' })
                .then(async (response) => {
                if (!response.ok) {
                    throw new Error(`HTTP ${response.status}`);
                }
                const text = await response.text();
                if (text.length > TEXT_PREVIEW_MAX_CHARS) {
                    setContent(text.slice(0, TEXT_PREVIEW_MAX_CHARS));
                    setTruncated(true);
                    return;
                }
                setContent(text);
            })
                .catch((err) => {
                if (err instanceof Error && err.name === 'AbortError') {
                    return;
                }
                setError(solidState5.t('BiChat.Artifacts.TextPreviewFailed'));
            })
                .finally(() => {
                setLoading(false);
            });
            return () => {
                controller.abort();
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    return <>{createMemo(() => {
            if (loading()) {
                return (<div class="flex min-h-[320px] items-center justify-center rounded-xl border border-gray-200 bg-gray-50 text-sm text-gray-500 dark:border-gray-700/60 dark:bg-gray-800/30 dark:text-gray-400">
        <SpinnerGap className="mr-2 h-4 w-4 animate-spin"/>
        {solidState5.t('BiChat.Artifacts.PreviewLoading')}
      </div>);
            }
            if (error()) {
                return <WarningBox message={error()!}/>;
            }
            return (<div class="space-y-2">
      <pre class="max-h-[70vh] overflow-auto rounded-xl border border-gray-200 bg-gray-50 p-3 text-xs leading-relaxed text-gray-800 dark:border-gray-700/60 dark:bg-gray-900 dark:text-gray-100">
        {content() || solidState5.t('BiChat.Artifacts.PreviewUnavailable')}
      </pre>
      {truncated() && (<p class="text-xs text-gray-500 dark:text-gray-400">{solidState5.t('BiChat.Artifacts.TextPreviewTruncated')}</p>)}
    </div>);
        })}</>;
}
export function SessionArtifactPreview(solidProps6: SessionArtifactPreviewProps) {
    const solidState7 = useTranslation();
    const artifactName = getArtifactName(solidProps6.artifact);
    const officeViewerURL = createMemo(() => {
        if (!solidProps6.artifact.url || !isAbsoluteHTTPURL(solidProps6.artifact.url)) {
            return null;
        }
        return `https://view.officeapps.live.com/op/embed.aspx?src=${encodeURIComponent(solidProps6.artifact.url)}`;
    });
    return <>{createMemo(() => {
            if (solidProps6.artifact.type === 'chart') {
                const chartData = parseChartDataFromArtifact(solidProps6.artifact);
                if (chartData) {
                    return <ChartCard chartData={chartData}/>;
                }
                return <WarningBox message={solidState7.t('BiChat.Artifacts.ChartUnavailable')}/>;
            }
            if (solidProps6.artifact.type === 'table' && solidProps6.artifact.metadata && typeof solidProps6.artifact.metadata === 'object') {
                const tableData = parseRenderTableDataFromMetadata(solidProps6.artifact.metadata as Record<string, unknown>, solidProps6.artifact.id);
                if (tableData) {
                    return (<div class="rounded-xl border border-gray-200/80 bg-white dark:border-gray-700/60 dark:bg-gray-900/30">
          <InteractiveTableCard table={tableData}/>
        </div>);
                }
                return <WarningBox message={solidState7.t('BiChat.Artifacts.PreviewUnavailable')}/>;
            }
            if (isImageArtifact(solidProps6.artifact)) {
                if (!solidProps6.artifact.url) {
                    return <WarningBox message={solidState7.t('BiChat.Artifacts.ImageUnavailable')}/>;
                }
                return (<div class="space-y-3">
        <div class="overflow-hidden rounded-xl border border-gray-200/80 bg-gray-50/50 dark:border-gray-700/60 dark:bg-gray-800/30">
          <img src={solidProps6.artifact.url} alt={artifactName} class="h-auto max-h-[72vh] w-full object-contain" loading="lazy"/>
        </div>
        <ArtifactActions url={solidProps6.artifact.url}/>
      </div>);
            }
            if (isPDFArtifact(solidProps6.artifact)) {
                if (!solidProps6.artifact.url) {
                    return <WarningBox message={solidState7.t('BiChat.Artifacts.DownloadUnavailable')}/>;
                }
                return (<div class="space-y-3">
        <div class="overflow-hidden rounded-xl border border-gray-200/80 bg-gray-50 dark:border-gray-700/60 dark:bg-gray-900">
          <iframe src={solidProps6.artifact.url} title={artifactName} class="h-[72vh] w-full"/>
        </div>
        <ArtifactActions url={solidProps6.artifact.url}/>
      </div>);
            }
            if (isOfficeDocumentArtifact(solidProps6.artifact)) {
                if (!solidProps6.artifact.url) {
                    return <WarningBox message={solidState7.t('BiChat.Artifacts.DownloadUnavailable')}/>;
                }
                return (<div class="space-y-3">
        {officeViewerURL() ? (<div class="overflow-hidden rounded-xl border border-gray-200/80 bg-gray-50 dark:border-gray-700/60 dark:bg-gray-900">
            <iframe src={officeViewerURL() ?? undefined} title={artifactName} class="h-[72vh] w-full"/>
          </div>) : (<WarningBox message={solidState7.t('BiChat.Artifacts.OfficePreviewUnavailable')}/>)}
        <ArtifactActions url={solidProps6.artifact.url}/>
      </div>);
            }
            if (isTextArtifact(solidProps6.artifact)) {
                return (<div class="space-y-3">
        <TextArtifactPreview artifact={solidProps6.artifact}/>
        {solidProps6.artifact.url && <ArtifactActions url={solidProps6.artifact.url}/>}
      </div>);
            }
            if (solidProps6.artifact.url) {
                return (<div class="space-y-3">
        <div class="flex min-h-[240px] flex-col items-center justify-center rounded-xl border border-gray-200/80 bg-gray-50/60 p-6 text-center dark:border-gray-700/60 dark:bg-gray-900">
          <FileText className="h-8 w-8 text-gray-400 dark:text-gray-500" weight="duotone"/>
          <p class="mt-3 text-sm font-medium text-gray-800 dark:text-gray-100">{solidState7.t('BiChat.Artifacts.PreviewUnavailable')}</p>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{solidState7.t('BiChat.Artifacts.PreviewNotSupported')}</p>
        </div>
        <ArtifactActions url={solidProps6.artifact.url}/>
      </div>);
            }
            return <WarningBox message={solidState7.t('BiChat.Artifacts.DownloadUnavailable')}/>;
        })}</>;
}
