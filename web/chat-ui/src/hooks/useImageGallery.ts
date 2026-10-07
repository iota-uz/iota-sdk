/** @jsxImportSource solid-js */
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * useImageGallery Hook
 * Manages image modal/gallery state and navigation
 */
import type { ImageAttachment } from '../types';
export interface UseImageGalleryOptions {
    /** Initial images to display */
    images?: ImageAttachment[];
    /** Wrap navigation at boundaries (default: false) */
    wrap?: boolean;
    /** Callback when modal opens */
    onOpen?: (index: number) => void;
    /** Callback when modal closes */
    onClose?: () => void;
    /** Callback when navigation occurs */
    onNavigate?: (index: number, direction: 'prev' | 'next') => void;
}
export interface UseImageGalleryReturn {
    /** Whether the gallery modal is open */
    isOpen: boolean;
    /** Current image index */
    currentIndex: number;
    /** Current image (or undefined if none) */
    currentImage: ImageAttachment | undefined;
    /** All images in the gallery */
    images: ImageAttachment[];
    /** Whether there's a previous image */
    hasPrev: boolean;
    /** Whether there's a next image */
    hasNext: boolean;
    /** Open gallery at specific index */
    open: (index: number, newImages?: ImageAttachment[]) => void;
    /** Close the gallery */
    close: () => void;
    /** Navigate to previous image */
    prev: () => void;
    /** Navigate to next image */
    next: () => void;
    /** Navigate to specific index */
    goTo: (index: number) => void;
    /** Set images without opening */
    setImages: (images: ImageAttachment[]) => void;
}
/**
 * Hook for managing image gallery/modal state
 *
 * @example
 * ```tsx
 * const gallery = useImageGallery({ images: attachments })
 *
 * // Open gallery
 * <button onClick={() => gallery.open(0)}>View Images</button>
 *
 * // Render gallery
 * {gallery.isOpen && (
 *   <ImageModal
 *     image={gallery.currentImage}
 *     onClose={gallery.close}
 *     onPrev={gallery.prev}
 *     onNext={gallery.next}
 *     hasPrev={gallery.hasPrev}
 *     hasNext={gallery.hasNext}
 *   />
 * )}
 * ```
 */
export function useImageGallery(options: UseImageGalleryOptions = {}): UseImageGalleryReturn {
    const { images: initialImages = [], wrap = false, onOpen, onClose, onNavigate } = options;
    const [isOpen, setIsOpen] = createSignal(false);
    const [currentIndex, setCurrentIndex] = createSignal(0);
    const [images, setImages] = createSignal<ImageAttachment[]>(initialImages);
    const currentImage = createMemo(() => images()[currentIndex()]);
    const hasPrev = createMemo(() => {
        if (wrap) {
            return images().length > 1;
        }
        return currentIndex() > 0;
    });
    const hasNext = createMemo(() => {
        if (wrap) {
            return images().length > 1;
        }
        return currentIndex() < images().length - 1;
    });
    const open = (index: number, newImages?: ImageAttachment[]) => {
        if (newImages) {
            setImages(newImages);
        }
        const targetImages = newImages || images();
        // Handle empty images array - don't compute negative index
        if (targetImages.length === 0) {
            setCurrentIndex(0);
            setIsOpen(true);
            return;
        }
        const safeIndex = Math.max(0, Math.min(index, targetImages.length - 1));
        setCurrentIndex(safeIndex);
        setIsOpen(true);
        onOpen?.(safeIndex);
    };
    const close = () => {
        setIsOpen(false);
        onClose?.();
    };
    const prev = () => {
        if (images().length < 2) {
            return;
        }
        if (!hasPrev() && !wrap) {
            return;
        }
        setCurrentIndex((current) => {
            const newIndex = wrap
                ? (current - 1 + images().length) % images().length
                : Math.max(0, current - 1);
            onNavigate?.(newIndex, 'prev');
            return newIndex;
        });
    };
    const next = () => {
        if (images().length < 2) {
            return;
        }
        if (!hasNext() && !wrap) {
            return;
        }
        setCurrentIndex((current) => {
            const newIndex = wrap ? (current + 1) % images().length : Math.min(images().length - 1, current + 1);
            onNavigate?.(newIndex, 'next');
            return newIndex;
        });
    };
    const goTo = (index: number) => {
        const safeIndex = Math.max(0, Math.min(index, images().length - 1));
        setCurrentIndex(safeIndex);
    };
    const setImagesHandler = (newImages: ImageAttachment[]) => {
        setImages(newImages);
        // Reset index if it's out of bounds (handle empty array case)
        if (newImages.length === 0) {
            setCurrentIndex(0);
        }
        else {
            setCurrentIndex((current) => Math.min(current, newImages.length - 1));
        }
    };
    return {
        get isOpen() {
            return isOpen();
        },
        get currentIndex() {
            return currentIndex();
        },
        get currentImage() {
            return currentImage();
        },
        get images() {
            return images();
        },
        get hasPrev() {
            return hasPrev();
        },
        get hasNext() {
            return hasNext();
        },
        open,
        close,
        prev,
        next,
        goTo,
        setImages: setImagesHandler,
    };
}
