import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * ImageModal Component
 * Full-screen image viewer with gallery navigation, zoom, and pan.
 * Uses the shadow-DOM-safe InlineDialog (NOT Headless UI Dialog, which portals
 * to document.body, escapes the shadow root, and loses all scoped Tailwind).
 */
import { InlineDialog, InlineDialogBackdrop, InlineDialogPanel } from './InlineDialog';
import { X, CaretLeft, CaretRight, ArrowClockwise, ArrowCounterClockwise, ImageBroken, MagnifyingGlassPlus, MagnifyingGlassMinus, ArrowsIn, } from '../icons';
import type { ImageAttachment } from '../types';
import { createDataUrl, formatFileSize } from '../utils/fileUtils';
import { useTranslation } from '../hooks/useTranslation';
interface ImageModalProps {
    isOpen: boolean;
    onClose: () => void;
    attachment: ImageAttachment;
    allAttachments?: ImageAttachment[];
    currentIndex?: number;
    onNavigate?: (direction: 'prev' | 'next') => void;
}
function ToolbarButton(solidProps1: {
    onClick: () => void;
    disabled?: boolean;
    'aria-label': string;
    children: JSX.Element;
}) {
    return (<button type="button" onClick={solidProps1.onClick} disabled={solidProps1.disabled} class="cursor-pointer flex items-center justify-center w-8 h-8 rounded-full text-white/70 hover:text-white hover:bg-white/10 transition-colors disabled:text-white/20 disabled:cursor-not-allowed disabled:hover:bg-transparent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/30" aria-label={solidProps1['aria-label']}>
      {solidProps1.children}
    </button>);
}
interface ViewerToolbarProps {
    scale: number;
    zoomPercent: number;
    isTransformed: boolean;
    onZoomIn: () => void;
    onZoomOut: () => void;
    onRotateLeft: () => void;
    onRotateRight: () => void;
    onReset: () => void;
    t: (key: string) => string;
}
function ViewerToolbar(solidProps2: ViewerToolbarProps) {
    return (<div class="absolute bottom-6 left-1/2 -translate-x-1/2 z-20 flex items-center gap-0.5 bg-black/50 backdrop-blur-xl rounded-full px-1.5 py-1.5 border border-white/10 shadow-2xl">
      <ToolbarButton onClick={solidProps2.onZoomOut} disabled={solidProps2.scale <= MIN_SCALE} aria-label={solidProps2.t('BiChat.Image.ZoomOut')}>
        <MagnifyingGlassMinus size={16} weight="bold"/>
      </ToolbarButton>

      <span class="text-xs text-white/60 tabular-nums font-medium min-w-[3.5rem] text-center select-none">
        {solidProps2.zoomPercent}%
      </span>

      <ToolbarButton onClick={solidProps2.onZoomIn} disabled={solidProps2.scale >= MAX_SCALE} aria-label={solidProps2.t('BiChat.Image.ZoomIn')}>
        <MagnifyingGlassPlus size={16} weight="bold"/>
      </ToolbarButton>

      <div class="w-px h-4 bg-white/15 mx-1"/>

      <ToolbarButton onClick={solidProps2.onRotateLeft} aria-label={solidProps2.t('BiChat.Image.RotateLeft')}>
        <ArrowCounterClockwise size={16} weight="bold"/>
      </ToolbarButton>

      <ToolbarButton onClick={solidProps2.onRotateRight} aria-label={solidProps2.t('BiChat.Image.RotateRight')}>
        <ArrowClockwise size={16} weight="bold"/>
      </ToolbarButton>

      {solidProps2.isTransformed && (<>
          <div class="w-px h-4 bg-white/15 mx-1"/>
          <ToolbarButton onClick={solidProps2.onReset} aria-label={solidProps2.t('BiChat.Image.ResetZoom')}>
            <ArrowsIn size={16} weight="bold"/>
          </ToolbarButton>
        </>)}
    </div>);
}
const MIN_SCALE = 0.25;
const MAX_SCALE = 5;
const ZOOM_STEP = 0.25;
function ImageModal(solidProps3Input: ImageModalProps) {
    const solidProps3 = mergeProps({ currentIndex: 0 } as const, solidProps3Input);
    const solidState4 = useTranslation();
    const [isImageLoaded, setIsImageLoaded] = createSignal(false);
    const [imageError, setImageError] = createSignal(false);
    const [retryKey, setRetryKey] = createSignal(0);
    // Zoom, pan & rotation state
    const [scale, setScale] = createSignal(1);
    const [position, setPosition] = createSignal({ x: 0, y: 0 });
    const [rotation, setRotation] = createSignal(0);
    const [isDragging, setIsDragging] = createSignal(false);
    const dragStartRef = { current: { x: 0, y: 0 } };
    const positionRef = { current: { x: 0, y: 0 } };
    const scaleRef = { current: 1 };
    const imageAreaRef = { current: null } as {
        current: HTMLDivElement | null;
    };
    const hasMultipleImages = createMemo(() => solidProps3.allAttachments && solidProps3.allAttachments.length > 1);
    const canNavigatePrev = createMemo(() => hasMultipleImages() && solidProps3.currentIndex > 0);
    const canNavigateNext = createMemo(() => hasMultipleImages() && solidProps3.currentIndex < (solidProps3.allAttachments?.length || 1) - 1);
    const isZoomed = createMemo(() => scale() > 1);
    const isTransformed = createMemo(() => isZoomed() || rotation() !== 0);
    // Keep refs in sync for event handlers
    createEffect(on(() => [scale()], () => {
        const cleanup = untrack(() => { scaleRef.current = scale(); });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    createEffect(on(() => [position()], () => {
        const cleanup = untrack(() => { positionRef.current = position(); });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const rotateLeft = () => {
        setRotation((r) => ((r - 90) % 360 + 360) % 360);
    };
    const rotateRight = () => {
        setRotation((r) => (r + 90) % 360);
    };
    // Keyboard navigation + zoom shortcuts
    createEffect(on(() => [solidProps3.isOpen, solidProps3.onNavigate, canNavigatePrev(), canNavigateNext(), rotateLeft, rotateRight], () => {
        const cleanup = untrack(() => {
            if (!solidProps3.isOpen) {
                return;
            }
            const handleKeyDown = (e: KeyboardEvent) => {
                if (e.key === 'ArrowLeft' && solidProps3.onNavigate && canNavigatePrev()) {
                    solidProps3.onNavigate?.('prev');
                }
                else if (e.key === 'ArrowRight' && solidProps3.onNavigate && canNavigateNext()) {
                    solidProps3.onNavigate?.('next');
                }
                else if (e.key === '+' || e.key === '=') {
                    setScale(s => Math.min(s + ZOOM_STEP, MAX_SCALE));
                }
                else if (e.key === '-') {
                    setScale(s => Math.max(s - ZOOM_STEP, MIN_SCALE));
                    if (scaleRef.current - ZOOM_STEP <= 1) {
                        setPosition({ x: 0, y: 0 });
                    }
                }
                else if (e.key === '0') {
                    setScale(1);
                    setPosition({ x: 0, y: 0 });
                    setRotation(0);
                }
                else if (e.key === 'r' && !e.shiftKey) {
                    rotateRight();
                }
                else if (e.key === 'R' || (e.key === 'r' && e.shiftKey)) {
                    rotateLeft();
                }
            };
            document.addEventListener('keydown', handleKeyDown);
            return () => document.removeEventListener('keydown', handleKeyDown);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Reset state on attachment change
    createEffect(on(() => [solidProps3.attachment], () => {
        const cleanup = untrack(() => {
            setIsImageLoaded(false);
            setImageError(false);
            setScale(1);
            setPosition({ x: 0, y: 0 });
            setRotation(0);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Mouse wheel zoom (needs native listener for preventDefault on passive)
    createEffect(on(() => [solidProps3.isOpen], () => {
        const cleanup = untrack(() => {
            const el = imageAreaRef.current;
            if (!el || !solidProps3.isOpen) {
                return;
            }
            const handler = (e: WheelEvent) => {
                const delta = e.deltaY > 0 ? -ZOOM_STEP : ZOOM_STEP;
                const current = scaleRef.current;
                const newScale = Math.min(Math.max(current + delta, MIN_SCALE), MAX_SCALE);
                if (newScale === current) {
                    return;
                }
                e.preventDefault();
                setScale(newScale);
                if (newScale <= 1) {
                    setPosition({ x: 0, y: 0 });
                }
            };
            el.addEventListener('wheel', handler, { passive: false });
            return () => el.removeEventListener('wheel', handler);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const handleRetry = () => {
        setImageError(false);
        setIsImageLoaded(false);
        setRetryKey((k) => k + 1);
    };
    // Zoom controls
    const zoomIn = () => {
        setScale(s => Math.min(s + ZOOM_STEP, MAX_SCALE));
    };
    const zoomOut = () => {
        setScale(s => Math.max(s - ZOOM_STEP, MIN_SCALE));
        if (scaleRef.current - ZOOM_STEP <= 1) {
            setPosition({ x: 0, y: 0 });
        }
    };
    const resetZoom = () => {
        setScale(1);
        setPosition({ x: 0, y: 0 });
        setRotation(0);
    };
    // Double-click to toggle between fit and 2x zoom (reset rotation when returning to 1x for consistency with resetZoom and '0' key)
    const handleDoubleClick = () => {
        const current = scaleRef.current;
        if (current !== 1) {
            setScale(1);
            setPosition({ x: 0, y: 0 });
            setRotation(0);
        }
        else {
            setScale(2);
        }
    };
    // Drag to pan (when zoomed)
    const handleMouseDown = (e: MouseEvent) => {
        if (scaleRef.current <= 1) {
            return;
        }
        e.preventDefault();
        setIsDragging(true);
        dragStartRef.current = {
            x: e.clientX - positionRef.current.x,
            y: e.clientY - positionRef.current.y,
        };
    };
    const handleMouseMove = (e: MouseEvent) => {
        if (!isDragging()) {
            return;
        }
        setPosition({
            x: e.clientX - dragStartRef.current.x,
            y: e.clientY - dragStartRef.current.y,
        });
    };
    const handleMouseUp = () => {
        setIsDragging(false);
    };
    // Click background to close (only when not zoomed; rotation alone does not block close)
    const handleBackdropClick = (e: MouseEvent) => {
        if (e.target === e.currentTarget && !isZoomed()) {
            solidProps3.onClose?.();
        }
    };
    const previewUrl = createMemo(() => solidProps3.attachment.preview || createDataUrl(solidProps3.attachment.base64Data, solidProps3.attachment.mimeType));
    const zoomPercent = createMemo(() => Math.round(scale() * 100));
    return (<InlineDialog open={solidProps3.isOpen} onClose={solidProps3.onClose} className="relative z-[99999]">
      <InlineDialogBackdrop className="fixed inset-0 bg-black/90 backdrop-blur-sm" style={{ "z-index": 99999 }}/>

      <InlineDialogPanel className="fixed inset-0 flex flex-col" style={{ "z-index": 100000 }} onMouseMove={handleMouseMove} onMouseUp={handleMouseUp} onMouseLeave={handleMouseUp}>
        {/* ── Header ── */}
        <div class="flex items-center px-5 py-3 shrink-0">
          <div class="flex items-center gap-3 min-w-0">
            {hasMultipleImages() && (<span class="text-xs text-white/50 tabular-nums whitespace-nowrap font-medium">
                {solidProps3.currentIndex + 1} / {solidProps3.allAttachments?.length}
              </span>)}
            <span class="text-sm text-white/90 truncate font-medium">{solidProps3.attachment.filename}</span>
            <span class="text-xs text-white/40 whitespace-nowrap">
              {formatFileSize(solidProps3.attachment.sizeBytes)}
            </span>
          </div>
        </div>

        {/* ── Image area ── */}
        <div ref={element => imageAreaRef.current = element} class="relative flex-1 flex items-center justify-center min-h-0 px-4 pb-4" onClick={handleBackdropClick} style={{ "cursor": isZoomed() ? (isDragging() ? 'grabbing' : 'grab') : 'default' }}>
          {/* ── Floating close button ── */}
          <button onClick={solidProps3.onClose} class="absolute top-3 right-5 z-30 cursor-pointer flex items-center justify-center w-10 h-10 rounded-full bg-black/50 hover:bg-black/70 backdrop-blur-md text-white/80 hover:text-white border border-white/10 transition-all duration-200 shadow-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/30" aria-label={solidState4.t('BiChat.Image.Close')} type="button">
            <X size={20} weight="bold"/>
          </button>

          {/* Loading spinner */}
          {!isImageLoaded() && !imageError() && (<div class="absolute inset-0 flex items-center justify-center pointer-events-none">
              <div class="flex flex-col items-center gap-3">
                <div class="w-8 h-8 border-2 border-white/20 border-t-white/60 rounded-full animate-spin"/>
                <span class="text-xs text-white/40">{solidState4.t('BiChat.Loading')}</span>
              </div>
            </div>)}

          {/* Error state */}
          {imageError() && (<div role="alert" class="flex flex-col items-center justify-center text-center max-w-xs">
              <div class="flex items-center justify-center w-16 h-16 rounded-2xl bg-white/5 border border-white/10 mb-5">
                <ImageBroken size={28} className="text-white/30" weight="duotone"/>
              </div>
              <p class="text-sm font-medium text-white/70 mb-1">{solidState4.t('BiChat.Image.FailedToLoad')}</p>
              <p class="text-xs text-white/30 mb-5 truncate max-w-full">{solidProps3.attachment.filename}</p>
              <button type="button" onClick={handleRetry} class="cursor-pointer inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-white/80 bg-white/10 hover:bg-white/15 border border-white/10 rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/30" aria-label={solidState4.t('BiChat.Image.Retry')}>
                <ArrowClockwise size={16} weight="bold"/>
                {solidState4.t('BiChat.Retry.Label')}
              </button>
            </div>)}

          {/* Image */}
          <img src={previewUrl()} alt={solidProps3.attachment.filename} class={[
            'relative z-0 max-w-[85vw] max-h-[calc(100vh-160px)] object-contain select-none rounded-lg',
            'transition-opacity duration-300 ease-out',
            isImageLoaded() ? 'opacity-100' : 'opacity-0',
        ].join(' ')} style={{
            "transform": `translate(${position().x}px, ${position().y}px) scale(${scale()}) rotate(${rotation()}deg)`,
            "transform-origin": 'center center',
            "transition": isDragging() ? 'opacity 0.3s ease-out'
                : 'transform 0.2s ease-out, opacity 0.3s ease-out'
        }} onLoad={() => setIsImageLoaded(true)} onError={() => setImageError(true)} onMouseDown={handleMouseDown} onDblClick={handleDoubleClick} loading="lazy" draggable={false}/>

          {/* ── Navigation arrows ── */}
          {hasMultipleImages() && (<>
              <button onClick={() => solidProps3.onNavigate?.('prev')} disabled={!canNavigatePrev() || !isImageLoaded() || imageError()} class={[
                'absolute left-4 top-1/2 -translate-y-1/2 z-20',
                'flex items-center justify-center w-11 h-11 rounded-full',
                'transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/30',
                canNavigatePrev() && isImageLoaded() && !imageError()
                    ? 'cursor-pointer bg-black/40 hover:bg-black/60 backdrop-blur-md text-white/80 hover:text-white shadow-lg border border-white/10'
                    : 'bg-black/20 text-white/20 cursor-not-allowed',
            ].join(' ')} aria-label={solidState4.t('BiChat.Image.Previous')} type="button">
                <CaretLeft size={20} weight="bold"/>
              </button>

              <button onClick={() => solidProps3.onNavigate?.('next')} disabled={!canNavigateNext() || !isImageLoaded() || imageError()} class={[
                'absolute right-4 top-1/2 -translate-y-1/2 z-20',
                'flex items-center justify-center w-11 h-11 rounded-full',
                'transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/30',
                canNavigateNext() && isImageLoaded() && !imageError()
                    ? 'cursor-pointer bg-black/40 hover:bg-black/60 backdrop-blur-md text-white/80 hover:text-white shadow-lg border border-white/10'
                    : 'bg-black/20 text-white/20 cursor-not-allowed',
            ].join(' ')} aria-label={solidState4.t('BiChat.Image.Next')} type="button">
                <CaretRight size={20} weight="bold"/>
              </button>
            </>)}

          {/* ── Zoom toolbar ── */}
          {isImageLoaded() && !imageError() && (<ViewerToolbar scale={scale()} zoomPercent={zoomPercent()} isTransformed={isTransformed()} onZoomIn={zoomIn} onZoomOut={zoomOut} onRotateLeft={rotateLeft} onRotateRight={rotateRight} onReset={resetZoom} t={solidState4.t}/>)}
        </div>
      </InlineDialogPanel>
    </InlineDialog>);
}
export { ImageModal };
export default ImageModal;
