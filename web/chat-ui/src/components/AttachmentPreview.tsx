import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * AttachmentPreview Component
 * Displays thumbnail preview of an image attachment
 * Shows filename, size, remove button, and supports click-to-enlarge
 */
import { X } from '../icons';
import { ImageAttachment } from '../types';
import { formatFileSize, createDataUrl } from '../utils/fileUtils';
import { useTranslation } from '../hooks/useTranslation';
interface AttachmentPreviewProps {
    /** The attachment to display */
    attachment: ImageAttachment;
    /** Optional callback when remove button is clicked */
    onRemove?: () => void;
    /** Optional callback when thumbnail is clicked (for enlargement) */
    onClick?: () => void;
    /** If true, hide remove button and disable click interactions */
    readonly?: boolean;
}
const AttachmentPreview = (solidProps1Input: AttachmentPreviewProps) => {
    const solidProps1 = mergeProps({ readonly: false } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const [isImageLoaded, setIsImageLoaded] = createSignal(false);
    const [imageError, setImageError] = createSignal(false);
    const previewUrl = createMemo(() => solidProps1.attachment.preview || createDataUrl(solidProps1.attachment.base64Data, solidProps1.attachment.mimeType));
    const isClickable = createMemo(() => solidProps1.onClick !== undefined && !solidProps1.readonly);
    const showRemoveButton = createMemo(() => solidProps1.onRemove !== undefined && !solidProps1.readonly);
    return (<div class={`
        relative
        rounded-lg
        border border-gray-200 dark:border-gray-700
        bg-white dark:bg-gray-800
        p-2
        transition-all
        duration-200
        ${isClickable() ? 'cursor-pointer hover:shadow-md hover:border-primary-400 dark:hover:border-primary-500' : ''}
        ${!isClickable() ? 'hover:shadow-sm' : ''}
      `} onClick={isClickable() ? solidProps1.onClick : undefined} role={isClickable() ? 'button' : undefined} tabIndex={isClickable() ? 0 : undefined} onKeyDown={isClickable() ? (e) => {
            if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                solidProps1.onClick?.();
            }
        }
            : undefined}>
      {/* Thumbnail Container */}
      <div class="relative mb-2 overflow-hidden rounded-md bg-gray-100 dark:bg-gray-700 aspect-square">
        {/* Loading Skeleton */}
        {!isImageLoaded() && !imageError() && (<div class="absolute inset-0 animate-pulse bg-gray-200 dark:bg-gray-600"/>)}

        {/* Error State */}
        {imageError() && (<div class="absolute inset-0 flex items-center justify-center bg-gray-100 dark:bg-gray-700">
            <span class="text-xs text-gray-500 dark:text-gray-400">{solidState2.t('BiChat.Attachment.PreviewUnavailable')}</span>
          </div>)}

        {/* Image */}
        <img src={previewUrl()} alt={solidProps1.attachment.filename} class={`
            w-full h-full object-cover
            transition-opacity duration-200
            ${isImageLoaded() ? 'opacity-100' : 'opacity-0'}
            ${isClickable() ? 'group-hover:scale-105' : ''}
          `} onLoad={() => setIsImageLoaded(true)} onError={() => setImageError(true)}/>
      </div>

      {/* Filename */}
      {!isImageLoaded() && !imageError() ? (<div class="h-3 w-3/4 bg-gray-200 dark:bg-gray-600 rounded animate-pulse mb-1"/>) : (<p class="text-xs font-medium text-gray-700 dark:text-gray-300 truncate" title={solidProps1.attachment.filename}>
          {solidProps1.attachment.filename}
        </p>)}

      {/* File Size */}
      {!isImageLoaded() && !imageError() ? (<div class="h-3 w-1/2 bg-gray-200 dark:bg-gray-600 rounded animate-pulse mb-1"/>) : (<p class="text-xs text-gray-500 dark:text-gray-400 mb-1">
          {formatFileSize(solidProps1.attachment.sizeBytes)}
        </p>)}

      {/* Remove Button */}
      {showRemoveButton() && (<button type="button" onClick={(e) => {
                e.stopPropagation();
                solidProps1.onRemove?.();
            }} class="absolute top-1 right-1 flex items-center justify-center bg-red-500 hover:bg-red-600 dark:bg-red-600 dark:hover:bg-red-700 text-white rounded-full transition-all duration-200 shadow-sm hover:shadow-md active:scale-90 w-6 h-6" aria-label={`Remove ${solidProps1.attachment.filename}`} title={solidState2.t('BiChat.Attachment.Remove')}>
          <X size={14} className="w-3.5 h-3.5" weight="bold"/>
        </button>)}
    </div>);
};
AttachmentPreview; /* Solid components are named by their declarations. */
export default AttachmentPreview;
