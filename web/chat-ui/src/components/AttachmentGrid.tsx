import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * AttachmentGrid Component
 * Displays image and non-image attachments as compact horizontal cards.
 */
import { X, Image as ImageIcon } from '../icons';
import { formatFileSize, getFileVisual } from '../utils/fileUtils';
import type { Attachment } from '../types';
interface AttachmentGridProps {
    attachments: Attachment[];
    onRemove?: (index: number) => void;
    onView?: (index: number) => void;
    className?: string;
    readonly?: boolean;
    maxDisplay?: number;
    maxCapacity?: number;
    emptyMessage?: string;
    showCount?: boolean;
    /** Number of files currently being processed (shows shimmer placeholders) */
    pendingCount?: number;
}
function isImageAttachment(attachment: Attachment): boolean {
    return attachment.mimeType.toLowerCase().startsWith('image/');
}
function resolveImagePreview(attachment: Attachment): string {
    if (attachment.preview) {
        return attachment.preview;
    }
    if (!isImageAttachment(attachment)) {
        return '';
    }
    if (attachment.base64Data) {
        if (attachment.base64Data.startsWith('data:')) {
            return attachment.base64Data;
        }
        return `data:${attachment.mimeType};base64,${attachment.base64Data}`;
    }
    return attachment.url || '';
}
/* ── Shared card styles ─────────────────────────────── */
const CARD_CLS = [
    'group relative flex items-center gap-2.5 rounded-xl',
    'border border-gray-200/80 dark:border-gray-700/60',
    'bg-white dark:bg-gray-800/60',
    'px-2.5 py-2',
    'transition-all duration-150',
].join(' ');
function RemoveButton(solidProps1: {
    index: number;
    onRemove: (i: number) => void;
    filename: string;
}) {
    return (<button type="button" onClick={(e) => { e.stopPropagation(); solidProps1.onRemove?.(solidProps1.index); }} class="flex-shrink-0 p-1 text-gray-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-md opacity-0 group-hover:opacity-100 transition-all duration-150 cursor-pointer focus-visible:outline-none focus-visible:opacity-100" aria-label={`Remove ${solidProps1.filename}`}>
      <X size={14} weight="bold"/>
    </button>);
}
/* ── Shimmer placeholder ───────────────────────────── */
function ShimmerCard() {
    const shimmerStyle = {
        "background": 'linear-gradient(110deg, transparent 30%, rgba(255,255,255,0.4) 50%, transparent 70%)',
        "background-size": '250% 100%',
        "animation": 'attachmentShimmer 1.5s ease-in-out infinite'
    };
    return (<div class={CARD_CLS} style={{ "pointer-events": 'none' }}>
      <div class="flex-shrink-0 w-10 h-10 rounded-lg bg-gray-100 dark:bg-gray-700 overflow-hidden">
        <div class="w-full h-full" style={shimmerStyle}/>
      </div>
      <div class="flex-1 space-y-1.5">
        <div class="h-3.5 w-28 rounded bg-gray-100 dark:bg-gray-700 overflow-hidden">
          <div class="w-full h-full" style={shimmerStyle}/>
        </div>
        <div class="h-3 w-16 rounded bg-gray-100 dark:bg-gray-700 overflow-hidden">
          <div class="w-full h-full" style={shimmerStyle}/>
        </div>
      </div>
      <style>{`
        @keyframes attachmentShimmer {
          0% { background-position: 200% 0; }
          100% { background-position: -60% 0; }
        }
      `}</style>
    </div>);
}
/* ── Image card ──────────────────────────────────────── */
interface ImageItemProps {
    attachment: Attachment;
    index: number;
    onRemove?: (index: number) => void;
    onView?: (index: number) => void;
}
function ImageItem(solidProps2: ImageItemProps) {
    const previewSrc = resolveImagePreview(solidProps2.attachment);
    const hasPreview = previewSrc !== '';
    const [imgFailed, setImgFailed] = createSignal(false);
    const thumbnail = createMemo(() => hasPreview && !imgFailed() ? (<img src={previewSrc} alt={solidProps2.attachment.filename} onError={() => setImgFailed(true)} class="w-10 h-10 rounded-lg object-cover bg-gray-100 dark:bg-gray-700"/>) : (<div class="flex items-center justify-center w-10 h-10 rounded-lg bg-violet-100 dark:bg-violet-900/40">
        <ImageIcon size={20} weight="duotone" className="text-violet-600 dark:text-violet-400"/>
      </div>));
    return (<div class={CARD_CLS}>
      {hasPreview && !imgFailed() && solidProps2.onView ? (<button type="button" onClick={() => solidProps2.onView?.(solidProps2.index)} class="flex-shrink-0 cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 rounded-lg hover:opacity-80 transition-opacity" aria-label={`View ${solidProps2.attachment.filename}`}>
          {thumbnail()}
        </button>) : (<div class="flex-shrink-0">{thumbnail()}</div>)}

      <div class="flex-1 min-w-0">
        <span class="block text-[13px] font-medium text-gray-900 dark:text-gray-100 truncate">
          {solidProps2.attachment.filename}
        </span>
        <span class="text-[11px] text-gray-400 dark:text-gray-500">
          {formatFileSize(solidProps2.attachment.sizeBytes)}
        </span>
      </div>

      {solidProps2.onRemove && <RemoveButton index={solidProps2.index} onRemove={solidProps2.onRemove} filename={solidProps2.attachment.filename}/>}
    </div>);
}
/* ── File card (non-image) ──────────────────────────── */
interface FileCardProps {
    attachment: Attachment;
    index: number;
    onRemove?: (index: number) => void;
}
function FileCard(solidProps3: FileCardProps) {
    const visual = getFileVisual(solidProps3.attachment.mimeType, solidProps3.attachment.filename);
    const Icon = visual.icon;
    return (<div class={CARD_CLS}>
      <div class={`flex-shrink-0 flex items-center justify-center w-10 h-10 rounded-lg ${visual.bgColor}`}>
        <Icon size={20} weight="duotone" className={visual.iconColor}/>
      </div>

      <div class="flex-1 min-w-0">
        <span class="block text-[13px] font-medium text-gray-900 dark:text-gray-100 truncate">
          {solidProps3.attachment.filename}
        </span>
        <span class="text-[11px] text-gray-400 dark:text-gray-500">
          {formatFileSize(solidProps3.attachment.sizeBytes)}
        </span>
      </div>

      {solidProps3.onRemove && <RemoveButton index={solidProps3.index} onRemove={solidProps3.onRemove} filename={solidProps3.attachment.filename}/>}
    </div>);
}
/* ── Memoization ───────────────────────────────────── */
const attachmentEq = (a: Attachment, b: Attachment) => a.clientKey === b.clientKey &&
    a.id === b.id &&
    a.filename === b.filename &&
    a.preview === b.preview &&
    a.base64Data === b.base64Data &&
    a.url === b.url;
const MemoizedImageItem = ImageItem;
const MemoizedFileCard = FileCard;
/* ── Grid ──────────────────────────────────────────── */
function AttachmentGrid(solidProps4Input: AttachmentGridProps) {
    const solidProps4 = mergeProps({ className: '', readonly: false, maxCapacity: 10, emptyMessage: 'No files attached', showCount: false, pendingCount: 0 } as const, solidProps4Input);
    const displayedAttachments = createMemo(() => solidProps4.maxDisplay && solidProps4.attachments.length > solidProps4.maxDisplay
        ? solidProps4.attachments.slice(0, solidProps4.maxDisplay)
        : solidProps4.attachments);
    const isAtMaxCapacity = createMemo(() => solidProps4.attachments.length >= solidProps4.maxCapacity);
    const hasContent = createMemo(() => displayedAttachments().length > 0 || solidProps4.pendingCount > 0);
    return <>{createMemo(() => {
            if (!hasContent()) {
                if (!solidProps4.showCount) {
                    return null;
                }
                return (<div class="text-center text-gray-500 dark:text-gray-400 py-4">{solidProps4.emptyMessage}</div>);
            }
            const isEditable = createMemo(() => !solidProps4.readonly && !!solidProps4.onRemove);
            return (<div class={`space-y-2 ${solidProps4.className}`}>
      {solidProps4.showCount && (<div class="text-sm text-gray-600 dark:text-gray-400">
          {displayedAttachments().length} file{displayedAttachments().length !== 1 ? 's' : ''} attached
        </div>)}

      <div class="grid gap-2">
        {displayedAttachments().map((attachment, index) => {
                    const isImage = isImageAttachment(attachment) && resolveImagePreview(attachment);
                    return isImage ? (<MemoizedImageItem attachment={attachment} index={index} onRemove={isEditable() ? solidProps4.onRemove : undefined} onView={solidProps4.onView}/>) : (<MemoizedFileCard attachment={attachment} index={index} onRemove={isEditable() ? solidProps4.onRemove : undefined}/>);
                })}

        {/* Shimmer placeholders for files being processed */}
        {Array.from({ length: solidProps4.pendingCount }).map((_, i) => (<ShimmerCard />))}
      </div>

      {solidProps4.maxDisplay && solidProps4.attachments.length > solidProps4.maxDisplay && (<div class="text-xs text-gray-500 dark:text-gray-400">
          +{solidProps4.attachments.length - solidProps4.maxDisplay} more
        </div>)}

      {isAtMaxCapacity() && isEditable() && (<div class="text-xs text-amber-600 dark:text-amber-400">
          Maximum {solidProps4.maxCapacity} files
        </div>)}
    </div>);
        })}</>;
}
const MemoizedAttachmentGrid = AttachmentGrid;
MemoizedAttachmentGrid; /* Solid components are named by their declarations. */
export { MemoizedAttachmentGrid as AttachmentGrid };
export default MemoizedAttachmentGrid;
