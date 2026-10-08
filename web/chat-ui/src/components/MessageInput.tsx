import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * MessageInput Component
 * Advanced input with file upload, drag-drop, keyboard shortcuts, and message queuing
 * Clean, professional design
 */
import { Menu, MenuButton, MenuItem, MenuItems } from './Menu';
import { Paperclip, PaperPlaneRight, X, Bug, ArrowUp, ArrowDown, Stack, Stop, Brain, CaretUpDown, Check } from '../icons';
import AttachmentGrid from './AttachmentGrid';
import { MessageQueueList } from './MessageQueueList';
import ImageModal from './ImageModal';
import { ATTACHMENT_ACCEPT_ATTRIBUTE, convertToBase64, createDataUrl, isImageMimeType, validateAttachmentFile, validateFileCount, } from '../utils/fileUtils';
import { calculateContextUsagePercent } from '../utils/debugMetrics';
import type { Attachment, ImageAttachment, DebugLimits, QueuedMessage, SessionDebugUsage } from '../types';
import { useTranslation } from '../hooks/useTranslation';
export interface MessageInputRef {
    focus: () => void;
    clear: () => void;
}
export interface MessageInputProps {
    message: string;
    loading: boolean;
    isStreaming?: boolean;
    fetching?: boolean;
    disabled?: boolean;
    commandError?: string | null;
    debugMode?: boolean;
    debugSessionUsage?: SessionDebugUsage;
    debugLimits?: DebugLimits | null;
    messageQueue?: QueuedMessage[];
    onClearCommandError?: () => void;
    onMessageChange: (value: string) => void;
    onSubmit: (e: Event, attachments: Attachment[]) => void;
    onCancelStreaming?: () => void;
    onUnqueue?: () => {
        content: string;
        attachments: Attachment[];
    } | null;
    onRemoveQueueItem?: (index: number) => void;
    onUpdateQueueItem?: (index: number, content: string) => void;
    placeholder?: string;
    maxFiles?: number;
    maxFileSize?: number;
    containerClassName?: string;
    formClassName?: string;
    reasoningEffortOptions?: string[];
    reasoningEffort?: string;
    onReasoningEffortChange?: (effort: string) => void;
}
/* -------------------------------------------------------------------------------------------------
 * DebugStatsPanel Sub-component
 * -----------------------------------------------------------------------------------------------*/
interface DebugStatsPanelProps {
    debugSessionUsage?: SessionDebugUsage;
    debugLimits?: DebugLimits | null;
}
/** Debug stats are English-only (developer-facing) — no i18n. */
function DebugStatsPanel(solidProps1: DebugStatsPanelProps) {
    const formatTokens = (value: number): string => new Intl.NumberFormat().format(value);
    const latestPromptTokens = solidProps1.debugSessionUsage?.latestPromptTokens ?? 0;
    const sessionTotalTokens = solidProps1.debugSessionUsage?.totalTokens ?? 0;
    const sessionPromptTokens = solidProps1.debugSessionUsage?.promptTokens ?? 0;
    const sessionCompletionTokens = solidProps1.debugSessionUsage?.completionTokens ?? 0;
    const hasUsage = (solidProps1.debugSessionUsage?.turnsWithUsage ?? 0) > 0;
    const policyMaxTokens = solidProps1.debugLimits?.policyMaxTokens ?? 0;
    const modelMaxTokens = solidProps1.debugLimits?.modelMaxTokens ?? 0;
    const effectiveMaxTokens = solidProps1.debugLimits?.effectiveMaxTokens ?? 0;
    const contextPercentValue = calculateContextUsagePercent(latestPromptTokens, effectiveMaxTokens);
    const contextPercent = contextPercentValue !== null ? contextPercentValue.toFixed(1) : null;
    const contextPercentNumber = parseFloat(contextPercent || '0');
    const contextBarColor = contextPercentNumber > 75
        ? '#ef4444'
        : contextPercentNumber > 50
            ? '#f59e0b'
            : '#10b981';
    return (<div class="mb-2 space-y-1.5 text-xs">
      {/* Debug badge */}
      <span class="inline-flex items-center gap-1.5 px-2 py-0.5 bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-300 rounded-full font-medium text-[10px]">
        <span class="relative flex h-1.5 w-1.5" aria-hidden="true">
          <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"/>
          <span class="relative inline-flex rounded-full h-1.5 w-1.5 bg-amber-500"/>
        </span>
        <Bug size={10}/>
        Debug
      </span>

      {/* Stats container */}
      <div class="rounded-lg border border-gray-200/60 dark:border-gray-700/40 bg-gray-50/50 dark:bg-gray-800/30 px-3 py-2 space-y-2">
        {hasUsage ? (<div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] tabular-nums">
            <span class="inline-flex items-center gap-1 text-gray-500 dark:text-gray-400">
              <ArrowUp size={10} weight="bold" className="text-blue-500 dark:text-blue-400"/>
              <span class="font-mono font-medium text-gray-700 dark:text-gray-300">{formatTokens(sessionPromptTokens)}</span>
              prompt
            </span>
            <span class="inline-flex items-center gap-1 text-gray-500 dark:text-gray-400">
              <ArrowDown size={10} weight="bold" className="text-indigo-500 dark:text-indigo-400"/>
              <span class="font-mono font-medium text-gray-700 dark:text-gray-300">{formatTokens(sessionCompletionTokens)}</span>
              completion
            </span>
            <span class="inline-flex items-center gap-1 text-gray-500 dark:text-gray-400">
              <Stack size={10} weight="bold" className="text-violet-500 dark:text-violet-400"/>
              <span class="font-mono font-medium text-gray-700 dark:text-gray-300">{formatTokens(sessionTotalTokens)}</span>
              total
            </span>
          </div>) : (<p class="text-[11px] text-gray-400 dark:text-gray-500 text-center py-0.5">
            Session usage unavailable
          </p>)}

        {solidProps1.debugLimits && (<div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] tabular-nums text-gray-500 dark:text-gray-400">
            <span>
              Policy max{' '}
              <span class="font-mono font-medium text-gray-700 dark:text-gray-300">{formatTokens(policyMaxTokens)}</span>
            </span>
            <span>
              Model max{' '}
              <span class="font-mono font-medium text-gray-700 dark:text-gray-300">{formatTokens(modelMaxTokens)}</span>
            </span>
            <span>
              Effective{' '}
              <span class="font-mono font-medium text-gray-700 dark:text-gray-300">{formatTokens(effectiveMaxTokens)}</span>
            </span>
          </div>)}

        {effectiveMaxTokens > 0 && (<div class="space-y-1.5">
            <div class="flex items-center justify-between">
              <span class="text-[10px] text-gray-400 dark:text-gray-500">
                Context usage
              </span>
              <div class="flex items-center gap-2">
                <span class="font-mono text-[10px] text-gray-400 dark:text-gray-500 tabular-nums">
                  {formatTokens(latestPromptTokens)} / {formatTokens(effectiveMaxTokens)}
                </span>
                {contextPercent && (<span class={[
                    'px-1.5 py-0.5 rounded-full text-[10px] font-semibold tabular-nums',
                    contextPercentNumber > 75
                        ? 'bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400'
                        : contextPercentNumber > 50
                            ? 'bg-amber-100 dark:bg-amber-900/30 text-amber-600 dark:text-amber-400'
                            : 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-600 dark:text-emerald-400',
                ].join(' ')}>
                    {contextPercent}%
                  </span>)}
              </div>
            </div>
            <div class="h-1.5 rounded-full bg-gray-200/80 dark:bg-gray-700/50 overflow-hidden">
              <div class="h-full rounded-full transition-all duration-700 ease-out" style={{
                "width": contextPercent ? `${Math.min(parseFloat(contextPercent), 100)}%` : '0%',
                "background-color": contextBarColor
            }}/>
            </div>
          </div>)}
      </div>
    </div>);
}
/* -------------------------------------------------------------------------------------------------
 * ReasoningEffortSelector Sub-component
 * -----------------------------------------------------------------------------------------------*/
const EFFORT_LABEL_KEYS: Record<string, string> = {
    low: 'BiChat.Input.ReasoningEffortLow',
    medium: 'BiChat.Input.ReasoningEffortMedium',
    high: 'BiChat.Input.ReasoningEffortHigh',
    xhigh: 'BiChat.Input.ReasoningEffortXHigh',
};
interface ReasoningEffortSelectorProps {
    options: string[];
    value?: string;
    onChange: (effort: string) => void;
    disabled?: boolean;
}
function ReasoningEffortSelector(solidProps2: ReasoningEffortSelectorProps) {
    const solidState3 = useTranslation();
    const selected = solidProps2.value || solidProps2.options[1] || solidProps2.options[0];
    const label = createMemo(() => solidState3.t('BiChat.Input.ReasoningEffort'));
    const selectedLabel = createMemo(() => solidState3.t(EFFORT_LABEL_KEYS[selected] ?? selected));
    return (<Menu as="div" className="relative flex-shrink-0 self-center">
      <MenuButton disabled={solidProps2.disabled} className={[
            'cursor-pointer inline-flex h-8 items-center gap-1.5 rounded-xl border border-gray-200/80 dark:border-gray-600/70',
            'bg-white dark:bg-gray-800 px-2.5 text-[11px] font-medium leading-none text-gray-700 dark:text-gray-200',
            'shadow-sm transition-colors hover:border-gray-300 hover:bg-white dark:hover:border-gray-500 dark:hover:bg-gray-800',
            'focus:outline-none focus:ring-2 focus:ring-primary-500/20',
            'disabled:cursor-not-allowed disabled:opacity-40',
        ].join(' ')} aria-label={label()} title={`${label()}: ${selectedLabel()}`}>
        <Brain size={14} weight="duotone" className="text-primary-600 dark:text-primary-400"/>
        <span class="max-w-[72px] truncate">{selectedLabel()}</span>
        <CaretUpDown size={12} className="text-gray-400 dark:text-gray-500"/>
      </MenuButton>

      <MenuItems anchor="top end" className="isolate z-30 min-w-[148px] rounded-xl border border-gray-200 bg-white p-1 shadow-xl ring-1 ring-black/5 dark:border-gray-700 dark:bg-gray-900 dark:ring-white/10 [--anchor-gap:8px]">
        {solidProps2.options.map((opt) => {
            const optionLabel = createMemo(() => solidState3.t(EFFORT_LABEL_KEYS[opt] ?? opt));
            const isSelected = opt === selected;
            return (<MenuItem>
              {(solidProps4) => (<button type="button" onClick={() => solidProps2.onChange?.(opt)} class={[
                        'flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left text-[11px] font-medium transition-colors',
                        solidProps4.focus ? 'bg-primary-50 text-primary-700 dark:bg-primary-950/40 dark:text-primary-200'
                            : 'text-gray-700 dark:text-gray-200',
                    ].join(' ')}>
                  <span class="flex-1 truncate">{optionLabel()}</span>
                  {isSelected && (<Check size={12} weight="bold" className="text-primary-600 dark:text-primary-400"/>)}
                </button>)}
            </MenuItem>);
        })}
      </MenuItems>
    </Menu>);
}
const MAX_FILES_DEFAULT = 10;
const MAX_FILE_SIZE_DEFAULT = 20 * 1024 * 1024; // 20MB
const MAX_HEIGHT = 192; // 12 lines approx
export const MessageInput = ((solidProps5Input: MessageInputProps & {
    ref?: (handle: MessageInputRef | undefined) => void;
}) => {
    const solidProps5 = mergeProps({ isStreaming: false, fetching: false, disabled: false, commandError: null, debugMode: false, debugLimits: null, messageQueue: [] as QueuedMessage[], maxFiles: MAX_FILES_DEFAULT, maxFileSize: MAX_FILE_SIZE_DEFAULT } as const, solidProps5Input);
    const solidState6 = useTranslation();
    const [attachments, setAttachments] = createSignal<Attachment[]>([]);
    const [isDragging, setIsDragging] = createSignal(false);
    const [error, setError] = createSignal<string | null>(null);
    const [isFocused, setIsFocused] = createSignal(false);
    const [commandListDismissed, setCommandListDismissed] = createSignal(false);
    const [activeCommandIndex, setActiveCommandIndex] = createSignal(0);
    const [isComposing, setIsComposing] = createSignal(false);
    const [dropSuccess, setDropSuccess] = createSignal(false);
    const [pendingFileCount, setPendingFileCount] = createSignal(0);
    const [viewingImageIndex, setViewingImageIndex] = createSignal<number | null>(null);
    // Use override or translation
    const placeholder = () => solidProps5.placeholder || solidState6.t('BiChat.Input.Placeholder');
    const textareaRef = { current: null } as {
        current: HTMLTextAreaElement | null;
    };
    const fileInputRef = { current: null } as {
        current: HTMLInputElement | null;
    };
    const containerRef = { current: null } as {
        current: HTMLDivElement | null;
    };
    const formRef = { current: null } as {
        current: HTMLFormElement | null;
    };
    const commandItemRefs = { current: [] } as {
        current: Array<HTMLLIElement | null>;
    };
    const didAutoFocusRef = { current: false };
    const isSlashMode = () => solidProps5.message.trimStart().startsWith('/');
    const commandQuery = () => solidProps5.message.trimStart().slice(1).split(/\s+/)[0]?.toLowerCase() || '';
    const slashCommands = [
        { name: '/clear', description: solidState6.t('BiChat.Slash.ClearDescription') },
        { name: '/debug', description: solidState6.t('BiChat.Slash.DebugDescription') },
        { name: '/compact', description: solidState6.t('BiChat.Slash.CompactDescription') },
    ];
    const filteredCommands = () => slashCommands.filter((cmd) => cmd.name.slice(1).startsWith(commandQuery()));
    const isCommandListVisible = () => isSlashMode() && !commandListDismissed() && !solidProps5.loading && !solidProps5.disabled;
    createEffect(on(() => [solidProps5.message], () => {
        const cleanup = untrack(() => {
            const textarea = textareaRef.current;
            if (!textarea) {
                return;
            }
            textarea.style.height = 'auto';
            const newHeight = Math.min(textarea.scrollHeight, MAX_HEIGHT);
            textarea.style.height = `${newHeight}px`;
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    createEffect(on(() => [solidProps5.loading, solidProps5.disabled, solidProps5.fetching], () => {
        const cleanup = untrack(() => {
            if (didAutoFocusRef.current || solidProps5.loading || solidProps5.disabled || solidProps5.fetching) {
                return;
            }
            const frame = requestAnimationFrame(() => {
                textareaRef.current?.focus();
                didAutoFocusRef.current = true;
            });
            return () => cancelAnimationFrame(frame);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    createEffect(on(() => [error()], () => {
        const cleanup = untrack(() => {
            if (!error()) {
                return;
            }
            const timer = setTimeout(() => setError(null), 5000);
            return () => clearTimeout(timer);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    createEffect(on(() => [isSlashMode(), solidProps5.message], () => {
        const cleanup = untrack(() => {
            if (isSlashMode()) {
                setCommandListDismissed(false);
            }
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    createEffect(on(() => [isCommandListVisible()], () => {
        const cleanup = untrack(() => {
            if (!isCommandListVisible()) {
                return;
            }
            const handleOutsideClick = (event: MouseEvent) => {
                if (!containerRef.current) {
                    return;
                }
                if (event.target instanceof Node && !containerRef.current.contains(event.target)) {
                    setCommandListDismissed(true);
                }
            };
            document.addEventListener('mousedown', handleOutsideClick);
            return () => document.removeEventListener('mousedown', handleOutsideClick);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    createEffect(on(() => [commandQuery()], () => {
        const cleanup = untrack(() => {
            setActiveCommandIndex(0);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    createEffect(on(() => [filteredCommands().length], () => {
        const cleanup = untrack(() => {
            if (filteredCommands().length === 0) {
                setActiveCommandIndex(0);
                return;
            }
            setActiveCommandIndex((prev) => {
                if (prev < 0) {
                    return 0;
                }
                if (prev >= filteredCommands().length) {
                    return filteredCommands().length - 1;
                }
                return prev;
            });
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    createEffect(on(() => [activeCommandIndex(), filteredCommands().length, isCommandListVisible()], () => {
        const cleanup = untrack(() => {
            if (!isCommandListVisible() || filteredCommands().length === 0) {
                return;
            }
            commandItemRefs.current[activeCommandIndex()]?.scrollIntoView({
                block: 'nearest',
            });
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const handleFileSelect = async (files: FileList | File[] | null): Promise<boolean> => {
        if (!files) {
            return false;
        }
        const selectedFiles = Array.isArray(files) ? files : Array.from(files);
        if (selectedFiles.length === 0) {
            return false;
        }
        try {
            validateFileCount(attachments().length, selectedFiles.length, solidProps5.maxFiles);
            // Extract and validate all files synchronously before any async work.
            // This ensures File objects are captured while the DataTransfer is still valid.
            const fileArray: File[] = [];
            for (let i = 0; i < selectedFiles.length; i++) {
                const file = selectedFiles[i];
                validateAttachmentFile(file, solidProps5.maxFileSize);
                fileArray.push(file);
            }
            // Show shimmer placeholders while processing
            setPendingFileCount(fileArray.length);
            // Read all files in parallel
            const base64Results = await Promise.all(fileArray.map(convertToBase64));
            const newAttachments: Attachment[] = fileArray.map((file, i) => {
                const attachment: Attachment = {
                    clientKey: crypto.randomUUID(),
                    filename: file.name,
                    mimeType: file.type,
                    sizeBytes: file.size,
                    base64Data: base64Results[i],
                };
                if (isImageMimeType(file.type)) {
                    attachment.preview = createDataUrl(base64Results[i], file.type);
                }
                return attachment;
            });
            setAttachments((prev) => [...prev, ...newAttachments]);
            setPendingFileCount(0);
            setError(null);
            return true;
        }
        catch (err) {
            setPendingFileCount(0);
            setError(err instanceof Error ? err.message : 'Failed to process attachments');
            return false;
        }
    };
    const handleFileInputChange = (e: Event & {
        currentTarget: HTMLInputElement;
    }) => {
        handleFileSelect(e.currentTarget.files);
        e.currentTarget.value = '';
    };
    const handle = ({
        focus: () => textareaRef.current?.focus(),
        clear: () => {
            solidProps5.onMessageChange?.('');
            setAttachments([]);
            setError(null);
        },
    });
    solidProps5.ref?.(handle);
    onCleanup(() => solidProps5.ref?.(undefined));
    const handleRemoveAttachment = (index: number) => {
        setAttachments((prev) => prev.filter((_, i) => i !== index));
        setError(null);
    };
    // ── Image lightbox ──────────────────────────────────
    const imageAttachments = attachments().filter((a): a is ImageAttachment => a.mimeType.startsWith('image/') && !!(a.base64Data && a.preview));
    const handleViewAttachment = (index: number) => {
        const attachment = attachments()[index];
        if (!attachment || !attachment.mimeType.startsWith('image/')) {
            return;
        }
        const imgIdx = imageAttachments.findIndex((a) => a.filename === attachment.filename && a.preview === attachment.preview);
        if (imgIdx >= 0) {
            setViewingImageIndex(imgIdx);
        }
    };
    const handleImageNavigate = (direction: 'prev' | 'next') => {
        setViewingImageIndex((prev) => {
            if (prev === null) {
                return null;
            }
            const len = imageAttachments.length;
            if (len <= 0) {
                return null;
            }
            const newIndex = Math.max(0, Math.min(len - 1, prev + (direction === 'prev' ? -1 : 1)));
            return newIndex;
        });
    };
    createEffect(on(() => [imageAttachments.length], () => {
        const cleanup = untrack(() => {
            setViewingImageIndex((prev) => {
                if (imageAttachments.length === 0) {
                    return null;
                }
                if (prev != null && (prev < 0 || prev >= imageAttachments.length)) {
                    return Math.max(0, Math.min(prev, imageAttachments.length - 1));
                }
                return prev;
            });
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // ── Paste-to-attach ─────────────────────────────────
    const handlePaste = (e: ClipboardEvent & {
        currentTarget: HTMLTextAreaElement;
    }) => {
        const items = Array.from(e.clipboardData?.items ?? []);
        const imageFiles = items
            .filter((item) => item.kind === 'file' && item.type.startsWith('image/'))
            .map((item) => item.getAsFile())
            .filter((file): file is File => file !== null);
        if (imageFiles.length > 0) {
            e.preventDefault();
            handleFileSelect(imageFiles);
        }
    }; // eslint-disable-line react-hooks/exhaustive-deps
    const handleDragOver = (e: DragEvent) => {
        e.preventDefault();
        e.stopPropagation();
        setIsDragging(true);
    };
    const handleDragLeave = (e: DragEvent) => {
        e.preventDefault();
        e.stopPropagation();
        setIsDragging(false);
    };
    const handleDrop = async (e: DragEvent) => {
        e.preventDefault();
        e.stopPropagation();
        setIsDragging(false);
        const itemFiles = Array.from(e.dataTransfer?.items ?? [])
            .filter((item) => item.kind === 'file')
            .map((item) => item.getAsFile())
            .filter((file): file is File => file !== null);
        const droppedFiles = itemFiles.length > 0 ? itemFiles : Array.from(e.dataTransfer?.files ?? []);
        const ok = await handleFileSelect(droppedFiles);
        if (ok) {
            setDropSuccess(true);
            setTimeout(() => setDropSuccess(false), 1500);
        }
    };
    const submitCommandSelection = (command: string) => {
        solidProps5.onMessageChange?.(command);
        setCommandListDismissed(true);
        setActiveCommandIndex(0);
        requestAnimationFrame(() => {
            formRef.current?.requestSubmit();
        });
    };
    const handleKeyDown = (e: KeyboardEvent) => {
        if (isComposing() || e.isComposing) {
            return;
        }
        if (isCommandListVisible()) {
            if (e.key === 'Tab') {
                e.preventDefault();
                if (filteredCommands().length > 0) {
                    solidProps5.onMessageChange?.(filteredCommands()[activeCommandIndex()].name);
                    setCommandListDismissed(true);
                }
                return;
            }
            if (e.key === 'ArrowDown') {
                e.preventDefault();
                if (filteredCommands().length > 0) {
                    setActiveCommandIndex((prev) => (prev + 1) % filteredCommands().length);
                }
                return;
            }
            if (e.key === 'ArrowUp') {
                e.preventDefault();
                if (filteredCommands().length > 0) {
                    setActiveCommandIndex((prev) => prev === 0 ? filteredCommands().length - 1 : prev - 1);
                }
                return;
            }
            if (e.key === 'Escape') {
                e.preventDefault();
                setCommandListDismissed(true);
                return;
            }
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                if (filteredCommands().length > 0) {
                    submitCommandSelection(filteredCommands()[activeCommandIndex()].name);
                    return;
                }
                handleFormSubmit(e as unknown as Event);
                return;
            }
        }
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            if (solidProps5.message.trim() || attachments().length > 0) {
                handleFormSubmit(e as unknown as Event);
            }
        }
        if (e.key === 'Escape') {
            if (isDragging()) {
                setIsDragging(false);
            }
            else if (isSlashMode()) {
                setCommandListDismissed(true);
            }
            else {
                textareaRef.current?.blur();
            }
        }
        if (e.key === 'ArrowUp' && !solidProps5.message.trim() && solidProps5.onUnqueue) {
            const unqueued = solidProps5.onUnqueue?.();
            if (unqueued) {
                solidProps5.onMessageChange?.(unqueued.content);
                setAttachments(unqueued.attachments);
            }
        }
    };
    const handleFormSubmit = (e: Event) => {
        e.preventDefault();
        if (isComposing()) {
            return;
        }
        if (solidProps5.disabled || (!solidProps5.message.trim() && attachments().length === 0)) {
            return;
        }
        setCommandListDismissed(true);
        solidProps5.onSubmit?.(e, attachments());
        setAttachments([]);
        setError(null);
    };
    const canSubmit = () => !solidProps5.disabled && (solidProps5.message.trim() || attachments().length > 0);
    const visibleError = () => error() || solidProps5.commandError;
    const visibleErrorText = () => visibleError() ? solidState6.t(visibleError()!) : '';
    const resolvedReasoningEffort = () => solidProps5.reasoningEffortOptions && solidProps5.reasoningEffortOptions.length > 0
        ? solidProps5.reasoningEffortOptions.includes(solidProps5.reasoningEffort ?? '')
            ? solidProps5.reasoningEffort : solidProps5.reasoningEffortOptions[1] || solidProps5.reasoningEffortOptions[0]
        : undefined;
    createEffect(on(() => [solidProps5.reasoningEffort, solidProps5.onReasoningEffortChange, solidProps5.reasoningEffortOptions, resolvedReasoningEffort()], () => {
        const cleanup = untrack(() => {
            if (!solidProps5.onReasoningEffortChange || !solidProps5.reasoningEffortOptions?.length) {
                return;
            }
            if (!resolvedReasoningEffort() || resolvedReasoningEffort() === solidProps5.reasoningEffort) {
                return;
            }
            solidProps5.onReasoningEffortChange?.(resolvedReasoningEffort()!);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const defaultContainerClassName = "shrink-0 px-4 pt-4 pb-6";
    return (<div ref={element => containerRef.current = element} class={solidProps5.containerClassName ?? defaultContainerClassName}>
        <form ref={element => formRef.current = element} onSubmit={handleFormSubmit} class={solidProps5.formClassName ?? "mx-auto"}>
          {/* Error display */}
          {visibleError() && (<div class="mb-3 flex items-start gap-2.5 px-3 py-2.5 bg-red-50 dark:bg-red-950/40 border border-red-200/80 dark:border-red-900/60 rounded-xl text-sm shadow-sm">
              <div class="flex-shrink-0 mt-0.5 flex items-center justify-center w-5 h-5 rounded-full bg-red-100 dark:bg-red-900/40">
                <X size={10} className="text-red-600 dark:text-red-400" weight="bold"/>
              </div>
              <span class="flex-1 text-red-700 dark:text-red-300 text-xs leading-relaxed">{visibleErrorText()}</span>
              <button type="button" onClick={() => {
                setError(null);
                solidProps5.onClearCommandError?.();
            }} class="cursor-pointer flex-shrink-0 p-0.5 text-red-400 dark:text-red-500 hover:text-red-600 dark:hover:text-red-300 hover:bg-red-100 dark:hover:bg-red-900/40 rounded-md transition-colors" aria-label={solidState6.t('BiChat.Input.DismissError')}>
                <X size={14}/>
              </button>
            </div>)}

          {/* Message queue list */}
          {solidProps5.messageQueue.length > 0 && solidProps5.onRemoveQueueItem && solidProps5.onUpdateQueueItem && (<MessageQueueList queue={solidProps5.messageQueue} onRemove={solidProps5.onRemoveQueueItem} onUpdate={solidProps5.onUpdateQueueItem}/>)}

          {solidProps5.debugMode && (<DebugStatsPanel debugSessionUsage={solidProps5.debugSessionUsage} debugLimits={solidProps5.debugLimits}/>)}

          {/* Attachment preview */}
          {(attachments().length > 0 || pendingFileCount() > 0) && (<div class="mb-3">
              <AttachmentGrid attachments={attachments()} onRemove={handleRemoveAttachment} onView={handleViewAttachment} pendingCount={pendingFileCount()}/>
            </div>)}

          {/* Input container with drag-drop */}
          <div class="relative" onDragOver={handleDragOver} onDragLeave={handleDragLeave} onDrop={handleDrop}>
            {/* Drag overlay */}
            {isDragging() && (<div class="absolute inset-0 z-10 bg-primary-50/95 dark:bg-primary-900/90 border-2 border-dashed border-primary-400 rounded-2xl flex items-center justify-center">
                <div class="flex flex-col items-center gap-2">
                  <div class="w-10 h-10 rounded-full bg-primary-100 dark:bg-primary-800 flex items-center justify-center">
                    <Paperclip size={20} className="text-primary-600 dark:text-primary-400"/>
                  </div>
                  <span class="text-sm text-primary-700 dark:text-primary-300 font-medium">
                    {solidState6.t('BiChat.Input.DropFiles')}
                  </span>
                </div>
              </div>)}

            {/* Drop success feedback */}
            {dropSuccess() && (<div class="absolute inset-0 z-10 bg-green-50/95 dark:bg-green-900/90 border-2 border-green-400 rounded-2xl flex items-center justify-center animate-pulse pointer-events-none">
                <span class="text-sm text-green-700 dark:text-green-300 font-medium">
                  {solidState6.t('BiChat.Input.FilesAdded')}
                </span>
              </div>)}

            {/* Input container - using inline Tailwind classes */}
            <div class={`flex items-center gap-2 rounded-2xl p-2 sm:p-3 bg-white dark:bg-gray-800 border shadow-sm transition-all duration-150 ${isFocused() ? 'border-primary-400 dark:border-primary-500 ring-2 ring-primary-500/20 dark:ring-primary-500/25 shadow-[0_0_0_3px_rgba(37,99,235,0.08)]'
            : 'border-gray-300 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-600 hover:shadow-md'}`}>
              {/* Attach button */}
              <button type="button" onClick={() => fileInputRef.current?.click()} disabled={solidProps5.loading || solidProps5.disabled || attachments().length >= solidProps5.maxFiles} class="cursor-pointer flex-shrink-0 self-center p-2 text-gray-500 dark:text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors disabled:opacity-40 disabled:cursor-not-allowed" aria-label={solidState6.t('BiChat.Input.AttachFiles')} title={solidState6.t('BiChat.Input.AttachFiles')}>
                <Paperclip size={20}/>
              </button>

              {/* Hidden file input */}
              <input ref={element => fileInputRef.current = element} type="file" accept={ATTACHMENT_ACCEPT_ATTRIBUTE} multiple onChange={handleFileInputChange} class="hidden" aria-label={solidState6.t('BiChat.Input.FileInput')}/>

              {/* Textarea */}
              <div class="flex-1 self-stretch flex items-center">
                <textarea data-testid="bichat-message-input" ref={element => textareaRef.current = element} value={solidProps5.message} onInput={(e) => {
            solidProps5.onMessageChange?.(e.currentTarget.value);
            solidProps5.onClearCommandError?.();
        }} onKeyDown={handleKeyDown} onPaste={handlePaste} onCompositionStart={() => setIsComposing(true)} onCompositionEnd={() => setIsComposing(false)} onFocus={() => setIsFocused(true)} onBlur={(e) => {
            setIsFocused(false);
            if (!containerRef.current) {
                return;
            }
            if (!e.relatedTarget || !containerRef.current.contains(e.relatedTarget as Node)) {
                setCommandListDismissed(true);
            }
        }} placeholder={placeholder()} class="resize-none bg-transparent border-none outline-none px-1 py-2 w-full text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 text-sm leading-relaxed" style={{ "max-height": `${MAX_HEIGHT}px` }} rows={1} disabled={solidProps5.disabled} aria-busy={solidProps5.loading} aria-label={solidState6.t('BiChat.Input.MessageInput')}/>
              </div>

              {/* Reasoning effort selector */}
              {solidProps5.reasoningEffortOptions && solidProps5.reasoningEffortOptions.length > 0 && solidProps5.onReasoningEffortChange && (<ReasoningEffortSelector options={solidProps5.reasoningEffortOptions} value={resolvedReasoningEffort()} onChange={solidProps5.onReasoningEffortChange} disabled={solidProps5.disabled || solidProps5.loading}/>)}

              {/* Submit/cancel button slot */}
              {solidProps5.isStreaming && solidProps5.onCancelStreaming ? (<button type="button" onClick={solidProps5.onCancelStreaming} disabled={solidProps5.disabled || solidProps5.fetching} class="cursor-pointer flex-shrink-0 self-center p-2 rounded-lg bg-gray-900 hover:bg-gray-800 active:bg-black active:scale-95 text-white shadow-sm transition-all dark:bg-gray-100 dark:hover:bg-gray-200 dark:active:bg-white dark:text-gray-900 disabled:opacity-40 disabled:cursor-not-allowed" aria-label={solidState6.t('BiChat.Common.Cancel')} title={solidState6.t('BiChat.Common.Cancel')}>
                  <Stop size={18} weight="fill"/>
                </button>) : (<button type="submit" disabled={!canSubmit()} class="cursor-pointer flex-shrink-0 self-center p-2 rounded-lg bg-primary-600 hover:bg-primary-700 active:bg-primary-800 active:scale-95 text-white shadow-sm transition-all disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:bg-primary-600" aria-label={solidProps5.loading ? solidState6.t('BiChat.Input.Processing') : solidState6.t('BiChat.Input.SendMessage')}>
                  {solidProps5.loading ? (<div class="w-[18px] h-[18px] border-2 border-white/60 border-t-transparent rounded-full animate-spin"/>) : (<PaperPlaneRight size={18} weight="fill"/>)}
                </button>)}
            </div>

            {/* Keyboard hint */}
            {isFocused() && !solidProps5.message && !solidProps5.loading && (<span class="hidden sm:block absolute -bottom-5 left-14 text-[10px] text-gray-400 dark:text-gray-500 select-none animate-fade-in">
                {solidState6.t('BiChat.Input.ShiftEnterHint')}
              </span>)}

            {isCommandListVisible() && (<div class="absolute left-0 right-0 bottom-full mb-1.5 z-20 overflow-hidden rounded-lg border border-gray-200/70 bg-white/98 shadow-md backdrop-blur-xl dark:border-gray-700/70 dark:bg-gray-900/98 dark:shadow-black/20">
                {filteredCommands().length > 0 ? (<ul role="listbox" aria-label={solidState6.t('BiChat.Slash.CommandsList')} class="py-1 px-1">
                    {filteredCommands().map((command, index) => {
                    const isActive = createMemo(() => index === activeCommandIndex());
                    return (<li role="option" aria-selected={isActive()} ref={(node) => {
                            commandItemRefs.current[index] = node;
                        }} onMouseEnter={() => setActiveCommandIndex(index)} onMouseDown={(e) => {
                            e.preventDefault();
                            submitCommandSelection(command.name);
                        }} class={`cursor-pointer flex items-baseline gap-2 rounded-md px-2 py-1.5 transition-colors duration-75 ${isActive() ? 'bg-gray-100 dark:bg-gray-800'
                            : 'hover:bg-gray-50 dark:hover:bg-gray-800/50'}`}>
                          <span class="text-xs font-medium font-mono text-gray-800 dark:text-gray-200 shrink-0">
                            <span class="text-gray-400 dark:text-gray-500">/</span>{command.name.slice(1)}
                          </span>
                          <span class="text-[11px] text-gray-400 dark:text-gray-500 truncate">
                            {command.description}
                          </span>
                        </li>);
                })}
                  </ul>) : (<div class="px-3 py-2.5 text-center">
                    <p class="text-[11px] text-gray-400 dark:text-gray-500">
                      {solidState6.t('BiChat.Slash.NoMatches')}
                    </p>
                  </div>)}
              </div>)}
          </div>

          {/* Image lightbox */}
          {viewingImageIndex() !== null &&
            viewingImageIndex()! >= 0 &&
            viewingImageIndex()! < imageAttachments.length && (<ImageModal isOpen onClose={() => setViewingImageIndex(null)} attachment={imageAttachments[viewingImageIndex()!]} allAttachments={imageAttachments} currentIndex={viewingImageIndex()!} onNavigate={handleImageNavigate}/>)}
        </form>
      </div>);
});
MessageInput; /* Solid components are named by their declarations. */
