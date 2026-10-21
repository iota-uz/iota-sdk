import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * UserMessage Component (Layer 3 Composite)
 * Styled component with slot-based customization for user messages
 */
import { Check, Copy, PencilSimple } from '../icons';
import { formatRelativeTime } from '../utils/dateFormatting';
import AttachmentGrid from './AttachmentGrid';
import ImageModal from './ImageModal';
import type { Attachment, ImageAttachment, UserTurn } from '../types';
import { useTranslation } from '../hooks/useTranslation';
/* -------------------------------------------------------------------------------------------------
 * Slot Props Types
 * -----------------------------------------------------------------------------------------------*/
export interface UserMessageAvatarSlotProps {
    /** Default initials */
    initials: string;
}
export interface UserMessageContentSlotProps {
    /** Message content text */
    content: string;
}
export interface UserMessageAttachmentsSlotProps {
    /** Message attachments */
    attachments: Attachment[];
    /** Handler to open image viewer */
    onView: (index: number) => void;
}
export interface UserMessageActionsSlotProps {
    /** Copy content to clipboard */
    onCopy: () => void;
    /** Edit message (if available) */
    onEdit?: () => void;
    /** Formatted timestamp */
    timestamp: string;
    /** Whether copy action is available */
    canCopy: boolean;
    /** Whether edit action is available */
    canEdit: boolean;
}
/* -------------------------------------------------------------------------------------------------
 * Component Types
 * -----------------------------------------------------------------------------------------------*/
export interface UserMessageSlots {
    /** Custom avatar renderer */
    avatar?: JSX.Element | ((props: UserMessageAvatarSlotProps) => JSX.Element);
    /** Custom content renderer */
    content?: JSX.Element | ((props: UserMessageContentSlotProps) => JSX.Element);
    /** Custom attachments renderer */
    attachments?: JSX.Element | ((props: UserMessageAttachmentsSlotProps) => JSX.Element);
    /** Custom actions renderer */
    actions?: JSX.Element | ((props: UserMessageActionsSlotProps) => JSX.Element);
}
export interface UserMessageClassNames {
    /** Root container */
    root?: string;
    /** Inner content wrapper */
    wrapper?: string;
    /** Avatar container */
    avatar?: string;
    /** Message bubble */
    bubble?: string;
    /** Content text */
    content?: string;
    /** Attachments container */
    attachments?: string;
    /** Actions container */
    actions?: string;
    /** Action button */
    actionButton?: string;
    /** Timestamp */
    timestamp?: string;
}
export interface UserMessageProps {
    /** User turn data */
    turn: UserTurn;
    /** Turn ID for edit operations */
    turnId?: string;
    /** User initials for avatar */
    initials?: string;
    /** Optional sender name for shared/group chats */
    authorName?: string;
    /** Slot overrides */
    slots?: UserMessageSlots;
    /** Class name overrides */
    classNames?: UserMessageClassNames;
    /** Copy handler */
    onCopy?: (content: string) => Promise<void> | void;
    /** Edit handler */
    onEdit?: (turnId: string, newContent: string) => void;
    /** Hide avatar */
    hideAvatar?: boolean;
    /** Hide actions */
    hideActions?: boolean;
    /** Hide timestamp */
    hideTimestamp?: boolean;
    /** Whether edit action should be available */
    allowEdit?: boolean;
}
const COPY_FEEDBACK_MS = 2000;
/* -------------------------------------------------------------------------------------------------
 * Default Styles
 * -----------------------------------------------------------------------------------------------*/
const defaultClassNames: Required<UserMessageClassNames> = {
    root: 'flex gap-3 justify-end group',
    wrapper: 'flex-1 min-w-0 flex flex-col items-end max-w-[var(--bichat-bubble-max-width)]',
    avatar: 'flex-shrink-0 w-8 h-8 rounded-full bg-primary-600 flex items-center justify-center text-white font-medium text-sm',
    bubble: 'bg-primary-600 text-white rounded-2xl rounded-br-sm px-4 py-3 shadow-sm',
    content: 'text-sm whitespace-pre-wrap break-words leading-relaxed',
    attachments: 'mb-2 w-full',
    actions: 'flex items-center gap-1 mt-2',
    actionButton: 'cursor-pointer p-2 min-h-[44px] min-w-[44px] flex items-center justify-center text-gray-500 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 active:bg-gray-200 dark:active:bg-gray-700 rounded-md transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50',
    timestamp: 'text-xs text-gray-400 dark:text-gray-500 mr-1',
};
function mergeClassNames(defaults: Required<UserMessageClassNames>, overrides?: UserMessageClassNames): Required<UserMessageClassNames> {
    if (!overrides) {
        return defaults;
    }
    return {
        root: overrides.root ?? defaults.root,
        wrapper: overrides.wrapper ?? defaults.wrapper,
        avatar: overrides.avatar ?? defaults.avatar,
        bubble: overrides.bubble ?? defaults.bubble,
        content: overrides.content ?? defaults.content,
        attachments: overrides.attachments ?? defaults.attachments,
        actions: overrides.actions ?? defaults.actions,
        actionButton: overrides.actionButton ?? defaults.actionButton,
        timestamp: overrides.timestamp ?? defaults.timestamp,
    };
}
/* -------------------------------------------------------------------------------------------------
 * EditForm Sub-component
 * -----------------------------------------------------------------------------------------------*/
interface EditFormProps {
    draftContent: string;
    onDraftChange: (e: Event & {
        currentTarget: HTMLTextAreaElement;
    }) => void;
    onSave: () => void;
    onCancel: () => void;
    onKeyDown: (e: KeyboardEvent) => void;
    textareaRef: {
        current: HTMLTextAreaElement | null;
    };
    disabled: boolean;
    originalContent: string;
    t: (key: string) => string;
}
function EditForm(solidProps1: EditFormProps) {
    return (<div class="space-y-3">
      <textarea ref={element => solidProps1.textareaRef.current = element} value={solidProps1.draftContent} onInput={solidProps1.onDraftChange} onKeyDown={solidProps1.onKeyDown} class="w-full min-h-[60px] max-h-[300px] resize-none rounded-xl border border-white/20 bg-white/[0.08] px-3.5 py-2.5 text-sm text-white leading-relaxed outline-none focus:bg-white/[0.12] focus:border-white/30 focus:ring-1 focus:ring-white/20 transition-all duration-200" aria-label={solidProps1.t('BiChat.Message.EditMessage')} rows={1}/>
      <div class="flex items-center justify-between gap-3">
        <span class="text-[11px] text-white/30 select-none hidden sm:inline">
          Esc · {typeof navigator !== 'undefined' && /mac|iphone|ipad/i.test((navigator as Navigator & {
            userAgentData?: {
                platform?: string;
            };
        }).userAgentData?.platform
            ?? navigator?.platform
            ?? '') ? '⌘' : 'Ctrl'}+Enter
        </span>
        <div class="flex items-center gap-2 ml-auto">
          <button type="button" onClick={solidProps1.onCancel} class="cursor-pointer px-3 py-1.5 rounded-lg text-white/60 hover:text-white hover:bg-white/10 transition-colors text-sm">
            {solidProps1.t('BiChat.Message.Cancel')}
          </button>
          <button type="button" onClick={solidProps1.onSave} class="cursor-pointer px-4 py-1.5 rounded-lg bg-white text-primary-700 font-medium text-sm hover:bg-white/90 transition-all shadow-sm disabled:opacity-40 disabled:cursor-not-allowed disabled:shadow-none" disabled={solidProps1.disabled || !solidProps1.draftContent.trim() || solidProps1.draftContent === solidProps1.originalContent}>
            {solidProps1.t('BiChat.Message.Save')}
          </button>
        </div>
      </div>
    </div>);
}
/* -------------------------------------------------------------------------------------------------
 * Component
 * -----------------------------------------------------------------------------------------------*/
export function UserMessage(solidProps2Input: UserMessageProps) {
    const solidProps2 = mergeProps({ initials: 'U', hideAvatar: false, hideActions: false, hideTimestamp: false, allowEdit: true } as const, solidProps2Input);
    const solidState3 = useTranslation();
    const [selectedImageIndex, setSelectedImageIndex] = createSignal<number | null>(null);
    const [isEditing, setIsEditing] = createSignal(false);
    const [draftContent, setDraftContent] = createSignal('');
    const [isCopied, setIsCopied] = createSignal(false);
    const copyFeedbackTimeoutRef = { current: null } as {
        current: (ReturnType<typeof setTimeout> | null) | null;
    };
    const editTextareaRef = { current: null } as {
        current: HTMLTextAreaElement | null;
    };
    const bubbleRef = { current: null } as {
        current: HTMLDivElement | null;
    };
    const classes = createMemo(() => mergeClassNames(defaultClassNames, solidProps2.classNames));
    onMount(() => {
        const cleanup = untrack(() => {
            return () => {
                if (copyFeedbackTimeoutRef.current) {
                    clearTimeout(copyFeedbackTimeoutRef.current);
                    copyFeedbackTimeoutRef.current = null;
                }
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    });
    // Reset edit state when the turn changes
    createEffect(on(() => [solidProps2.turnId], () => {
        const cleanup = untrack(() => {
            setIsEditing(false);
            setDraftContent('');
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Auto-focus textarea when entering edit mode
    createEffect(on(() => [isEditing()], () => {
        const cleanup = untrack(() => {
            if (isEditing() && editTextareaRef.current) {
                const textarea = editTextareaRef.current;
                textarea.focus();
                textarea.selectionStart = textarea.value.length;
                textarea.selectionEnd = textarea.value.length;
                textarea.style.height = 'auto';
                textarea.style.height = `${Math.min(textarea.scrollHeight, 300)}px`;
            }
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Click-outside to cancel edit
    createEffect(on(() => [isEditing()], () => {
        const cleanup = untrack(() => {
            if (!isEditing()) {
                return;
            }
            const handleMouseDown = (e: MouseEvent) => {
                if (bubbleRef.current && !bubbleRef.current.contains(e.target as Node)) {
                    setIsEditing(false);
                    setDraftContent('');
                }
            };
            document.addEventListener('mousedown', handleMouseDown);
            return () => document.removeEventListener('mousedown', handleMouseDown);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const normalizedAttachments = createMemo<Attachment[]>(() => solidProps2.turn.attachments.map((attachment) => {
        if (!attachment.mimeType.startsWith('image/')) {
            return attachment;
        }
        if (attachment.preview) {
            return attachment;
        }
        if (attachment.base64Data) {
            if (attachment.base64Data.startsWith('data:')) {
                return {
                    ...attachment,
                    preview: attachment.base64Data,
                };
            }
            return {
                ...attachment,
                preview: `data:${attachment.mimeType};base64,${attachment.base64Data}`,
            };
        }
        if (attachment.url) {
            return {
                ...attachment,
                preview: attachment.url,
            };
        }
        return attachment;
    }));
    const solidState4 = createMemo(() => {
        const images: ImageAttachment[] = [];
        const indexMap = new Map<number, number>();
        normalizedAttachments().forEach((attachment, index) => {
            if (!attachment.mimeType.startsWith('image/')) {
                return;
            }
            if (!attachment.preview && !attachment.url) {
                return;
            }
            indexMap.set(index, images.length);
            images.push({
                ...attachment,
                base64Data: attachment.base64Data || '',
                preview: attachment.preview || attachment.url || '',
            });
        });
        return { imageAttachments: images, imageIndexByAttachmentIndex: indexMap };
    });
    const handleCopyClick = async () => {
        try {
            if (solidProps2.onCopy) {
                await solidProps2.onCopy?.(solidProps2.turn.content);
            }
            else {
                await navigator.clipboard.writeText(solidProps2.turn.content);
            }
            setIsCopied(true);
            if (copyFeedbackTimeoutRef.current) {
                clearTimeout(copyFeedbackTimeoutRef.current);
            }
            copyFeedbackTimeoutRef.current = setTimeout(() => {
                setIsCopied(false);
                copyFeedbackTimeoutRef.current = null;
            }, COPY_FEEDBACK_MS);
        }
        catch (err) {
            setIsCopied(false);
            console.error('Failed to copy:', err);
        }
    };
    const handleEditClick = () => {
        if (solidProps2.onEdit && solidProps2.turnId) {
            setDraftContent(solidProps2.turn.content);
            setIsEditing(true);
        }
    };
    const handleEditCancel = () => {
        setIsEditing(false);
        setDraftContent('');
    };
    const handleEditSave = () => {
        if (!solidProps2.onEdit || !solidProps2.turnId) {
            return;
        }
        const newContent = draftContent();
        if (!newContent.trim()) {
            return;
        }
        if (newContent === solidProps2.turn.content) {
            setIsEditing(false);
            return;
        }
        solidProps2.onEdit?.(solidProps2.turnId, newContent);
        setIsEditing(false);
    };
    const handleEditKeyDown = (e: KeyboardEvent) => {
        if (e.key === 'Escape') {
            e.preventDefault();
            handleEditCancel();
        }
        else if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
            e.preventDefault();
            handleEditSave();
        }
    };
    const handleDraftChange = (e: Event & {
        currentTarget: HTMLTextAreaElement;
    }) => {
        setDraftContent(e.currentTarget.value);
        const el = e.currentTarget;
        el.style.height = 'auto';
        el.style.height = `${Math.min(el.scrollHeight, 300)}px`;
    };
    const handleNavigate = (direction: 'prev' | 'next') => {
        const index = selectedImageIndex();
        if (index === null) {
            return;
        }
        if (direction === 'prev' && index > 0) {
            setSelectedImageIndex(index - 1);
        }
        else if (direction === 'next' && index < solidState4().imageAttachments.length - 1) {
            setSelectedImageIndex(index + 1);
        }
    };
    const currentAttachment = createMemo(() => { const index = selectedImageIndex(); return index !== null ? solidState4().imageAttachments[index] : null; });
    const timestamp = createMemo(() => formatRelativeTime(solidProps2.turn.createdAt, solidState3.t));
    // Slot props
    const avatarSlotProps: UserMessageAvatarSlotProps = { get initials() {
            return solidProps2.initials;
        } };
    const contentSlotProps = createMemo<UserMessageContentSlotProps>(() => ({ content: solidProps2.turn.content }));
    const attachmentsSlotProps = createMemo<UserMessageAttachmentsSlotProps>(() => ({
        attachments: normalizedAttachments(),
        onView: (index) => {
            const imageIndex = solidState4().imageIndexByAttachmentIndex.get(index);
            if (imageIndex === undefined) {
                return;
            }
            setSelectedImageIndex(imageIndex);
        },
    }));
    const actionsSlotProps = createMemo<UserMessageActionsSlotProps>(() => ({
        onCopy: handleCopyClick,
        onEdit: solidProps2.onEdit && solidProps2.turnId && solidProps2.allowEdit ? handleEditClick : undefined,
        get timestamp() {
            return timestamp();
        },
        canCopy: true,
        canEdit: !!solidProps2.onEdit && !!solidProps2.turnId && solidProps2.allowEdit,
    }));
    // Render helpers
    const renderSlot = <T,>(slot: JSX.Element | ((props: T) => JSX.Element) | undefined, props: T, defaultContent: JSX.Element): JSX.Element => {
        if (slot === undefined) {
            return defaultContent;
        }
        if (typeof slot === 'function') {
            return slot(props);
        }
        return slot;
    };
    return (<div class={classes().root}>
      <div class={classes().wrapper}>
        {solidProps2.authorName && (<span id={`${solidProps2.turn.id}-author`} role="note" aria-label={solidProps2.authorName} class="mb-1 px-1 text-[11px] text-right text-gray-500 dark:text-gray-400">
            {solidProps2.authorName}
          </span>)}
        {/* Attachments */}
        {normalizedAttachments().length > 0 && (<div class={classes().attachments}>
            {renderSlot(solidProps2.slots?.attachments, attachmentsSlotProps(), <AttachmentGrid attachments={normalizedAttachments()} onView={attachmentsSlotProps().onView}/>)}
          </div>)}

        {/* Message bubble */}
        {solidProps2.turn.content && (<div ref={element => bubbleRef.current = element} class={classes().bubble} aria-describedby={solidProps2.authorName ? `${solidProps2.turn.id}-author` : undefined}>
            <div class={classes().content}>
              {isEditing() ? (<EditForm draftContent={draftContent()} onDraftChange={handleDraftChange} onSave={handleEditSave} onCancel={handleEditCancel} onKeyDown={handleEditKeyDown} textareaRef={editTextareaRef} disabled={false} originalContent={solidProps2.turn.content} t={solidState3.t}/>) : (renderSlot(solidProps2.slots?.content, contentSlotProps(), solidProps2.turn.content))}
            </div>
          </div>)}

        {/* Actions */}
        {!solidProps2.hideActions && (<div class={`${classes().actions} ${isCopied() ? 'opacity-100' : ''}`}>
            {renderSlot(solidProps2.slots?.actions, actionsSlotProps(), <>
                {!solidProps2.hideTimestamp && <span class={classes().timestamp}>{timestamp()}</span>}

                <button onClick={handleCopyClick} class={`cursor-pointer ${classes().actionButton} ${isCopied() ? 'text-green-600 dark:text-green-400' : ''}`} aria-label={solidState3.t('BiChat.Message.CopyMessage')} title={isCopied() ? solidState3.t('BiChat.Message.Copied') : solidState3.t('BiChat.Message.Copy')} data-copied={isCopied() ? 'true' : undefined}>
                  {isCopied() ? <Check size={14} weight="bold"/> : <Copy size={14} weight="regular"/>}
                </button>

                {solidProps2.onEdit && solidProps2.turnId && solidProps2.allowEdit && (<button onClick={handleEditClick} class={`cursor-pointer ${classes().actionButton}`} aria-label={solidState3.t('BiChat.Message.EditMessage')} title={solidState3.t('BiChat.Message.EditMessage')} disabled={isEditing()}>
                    <PencilSimple size={14} weight="regular"/>
                  </button>)}
              </>)}
          </div>)}
      </div>

      {/* Avatar */}
      {!solidProps2.hideAvatar && (<div class={classes().avatar}>
          {renderSlot(solidProps2.slots?.avatar, avatarSlotProps, solidProps2.initials)}
        </div>)}

      {/* Image modal */}
      {currentAttachment() && (<ImageModal isOpen={selectedImageIndex() !== null} onClose={() => setSelectedImageIndex(null)} attachment={currentAttachment()!} allAttachments={solidState4().imageAttachments} currentIndex={selectedImageIndex() ?? 0} onNavigate={handleNavigate}/>)}
    </div>);
}
export default UserMessage;
