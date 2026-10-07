import { createHorizontalSwipe, type SwipeInfo } from '../hooks/createHorizontalSwipe';
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { Menu, MenuButton, MenuItem, MenuItems } from './Menu';
import { DotsThree, Check, Bookmark, PencilSimple, Archive, ArrowsClockwise, ArrowUUpLeft, Trash, UsersThree } from '../icons';
import { EditableText, type EditableTextRef } from './EditableText';
import { sessionItemVariants } from '../animations/variants';
import type { Session } from '../types';
import { useLongPress } from '../hooks/useLongPress';
import { TouchContextMenu, type ContextMenuItem } from './TouchContextMenu';
import { useTranslation } from '../hooks/useTranslation';
import { formatRelativeTime } from '../utils/dateFormatting';
interface SessionItemProps {
    session: Session;
    isActive: boolean;
    mode?: 'active' | 'archived';
    onSelect: (sessionId: string) => void;
    onArchive?: () => void;
    onRestore?: () => void;
    onPin?: () => void;
    onRename?: (newTitle: string) => void;
    onRegenerateTitle?: () => void;
    onDelete?: () => void;
    testIdPrefix?: string;
    className?: string;
}
const SessionItem = (solidProps1Input: SessionItemProps) => {
    const solidProps1 = mergeProps({ mode: 'active', testIdPrefix: 'sidebar', className: '' } as const, solidProps1Input);
    const editableTitleRef = { current: null } as {
        current: EditableTextRef | null;
    };
    const itemRef = { current: null } as {
        current: HTMLDivElement | null;
    };
    const [menuOpen, setMenuOpen] = createSignal(false);
    const [menuAnchor, setMenuAnchor] = createSignal<DOMRect | null>(null);
    const [isTouch, setIsTouch] = createSignal(false);
    const solidState2 = useTranslation();
    // Drag-to-archive gesture
    const isDraggingRef = { current: false };
    const archiveGesture = createHorizontalSwipe({
        enabled: () => Boolean(solidProps1.onArchive),
        left: -100,
        onStart: () => { isDraggingRef.current = true; },
        onEnd: (event, info) => handleDragEnd(event, info),
    });
    const archiveOpacity = createMemo(() => Math.min(1, -archiveGesture.offset() / 80));
    const archiveScale = createMemo(() => 0.6 + archiveOpacity() * 0.4);
    const canDragArchive = createMemo(() => !!solidProps1.onArchive);
    const handleDragEnd = (_: MouseEvent | TouchEvent | PointerEvent, info: SwipeInfo) => {
        if (info.offset.x < -80 && solidProps1.onArchive) {
            solidProps1.onArchive?.();
        }
        // Defer reset so the click event (which fires synchronously after dragEnd)
        // still sees isDraggingRef=true and skips navigation.
        requestAnimationFrame(() => {
            isDraggingRef.current = false;
        });
    };
    // Detect touch device
    onMount(() => {
        const cleanup = untrack(() => {
            setIsTouch('ontouchend' in document);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    });
    // Only treat empty/whitespace title as generating.
    // Non-empty placeholders should be displayed as-is.
    const isTitleGenerating = createMemo(() => !solidProps1.session.title?.trim());
    // Generate title from session (use existing title or show generating state)
    const displayTitle = createMemo(() => isTitleGenerating() ? solidState2.t('BiChat.Common.Generating') : (solidProps1.session.title ?? solidState2.t('BiChat.Common.Untitled')));
    const lastActivity = createMemo(() => formatRelativeTime(solidProps1.session.updatedAt, solidState2.t));
    const accessRole = createMemo(() => solidProps1.session.access?.role ?? 'owner');
    const roleLabel = createMemo(() => accessRole() === 'editor'
        ? solidState2.t('BiChat.Share.RoleEditor')
        : accessRole() === 'viewer'
            ? solidState2.t('BiChat.Share.RoleViewer')
            : accessRole() === 'read_all'
                ? solidState2.t('BiChat.Share.RoleReadOnly')
                : '');
    const visibilityLabel = createMemo(() => solidProps1.session.isGroup
        ? solidState2.t('BiChat.Sidebar.GroupChat')
        : accessRole() === 'editor' || accessRole() === 'viewer'
            ? solidState2.t('BiChat.Sidebar.SharedWithYou')
            : '');
    const isGroupOrShared = createMemo(() => Boolean(solidProps1.session.isGroup || (solidProps1.session.memberCount && solidProps1.session.memberCount > 1)));
    const metaParts = createMemo(() => [lastActivity(), visibilityLabel(), roleLabel()].filter(Boolean));
    // Long press handlers for touch devices
    const solidState3 = useLongPress({
        delay: 500,
        onLongPress: (e) => {
            const target = e.currentTarget as HTMLElement;
            setMenuAnchor(target.getBoundingClientRect());
            setMenuOpen(true);
        },
        hapticFeedback: true,
    });
    // Add contextmenu event listener as fallback for iPadOS
    createEffect(on(() => [itemRef], () => {
        const cleanup = untrack(() => {
            const element = itemRef.current;
            if (!element) {
                return;
            }
            const isIPad = /iPad|Macintosh/i.test(navigator.userAgent) && 'ontouchend' in document;
            if (!isIPad) {
                return;
            }
            const handleContextMenu = (e: Event) => {
                e.preventDefault();
                const target = e.currentTarget as HTMLElement;
                setMenuAnchor(target.getBoundingClientRect());
                setMenuOpen(true);
            };
            element.addEventListener('contextmenu', handleContextMenu);
            return () => element.removeEventListener('contextmenu', handleContextMenu);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const contextMenuItems = createMemo<ContextMenuItem[]>(() => solidProps1.mode === 'archived'
        ? [
            ...(solidProps1.onRestore ? [{
                    id: 'restore',
                    label: solidState2.t('BiChat.Archived.RestoreButton'),
                    icon: <ArrowUUpLeft size={20}/>,
                    onClick: () => solidProps1.onRestore?.(),
                }] : []),
            ...(solidProps1.onRename ? [{
                    id: 'rename',
                    label: solidState2.t('BiChat.Sidebar.RenameChat'),
                    icon: <PencilSimple size={20}/>,
                    onClick: () => editableTitleRef.current?.startEditing(),
                }] : []),
        ]
        : [
            ...(solidProps1.onPin ? [{
                    id: 'pin',
                    label: solidProps1.session.pinned ? solidState2.t('BiChat.Sidebar.UnpinChat') : solidState2.t('BiChat.Sidebar.PinChat'),
                    icon: solidProps1.session.pinned ? <Check size={20}/> : <Bookmark size={20}/>,
                    onClick: () => solidProps1.onPin?.(),
                }] : []),
            ...(solidProps1.onRename ? [{
                    id: 'rename',
                    label: solidState2.t('BiChat.Sidebar.RenameChat'),
                    icon: <PencilSimple size={20}/>,
                    onClick: () => editableTitleRef.current?.startEditing(),
                }] : []),
            ...(solidProps1.onRegenerateTitle ? [{
                    id: 'regenerate',
                    label: solidState2.t('BiChat.Sidebar.RegenerateTitle'),
                    icon: <ArrowsClockwise size={20}/>,
                    onClick: () => solidProps1.onRegenerateTitle?.(),
                }] : []),
            ...(solidProps1.onArchive ? [{
                    id: 'archive',
                    label: solidState2.t('BiChat.Sidebar.ArchiveChat'),
                    icon: <Archive size={20}/>,
                    onClick: () => solidProps1.onArchive?.(),
                    variant: 'danger' as const,
                }] : []),
            ...(solidProps1.onDelete ? [{
                    id: 'delete',
                    label: solidState2.t('BiChat.Sidebar.DeleteChat'),
                    icon: <Trash size={20}/>,
                    onClick: () => solidProps1.onDelete?.(),
                    variant: 'danger' as const,
                }] : []),
        ]);
    const hasContextMenu = createMemo(() => contextMenuItems().length > 0);
    return (<>
        <div class="relative overflow-hidden rounded-lg">
          {/* Archive zone — revealed by drag */}
          {canDragArchive() && (<div class="absolute inset-y-0 right-0 w-20 flex items-center justify-center bg-gray-500 dark:bg-gray-600 rounded-r-lg" style={{ "opacity": archiveOpacity(), "scale": archiveScale() }} aria-hidden="true">
              <Archive size={20} className="text-white"/>
            </div>)}

          {/* Draggable item content */}
          <div {...archiveGesture.handlers} style={{ "transform": `translateX(${archiveGesture.offset()}px)`, "touch-action": "pan-y" }} class="relative">
          <div role="button" tabIndex={0} ref={element => itemRef.current = element} onClick={() => {
            if (isDraggingRef.current) {
                return;
            }
            solidProps1.onSelect?.(solidProps1.session.id);
        }} onKeyDown={(e) => {
            const target = e.target as HTMLElement | null;
            const isFromEditable = !!target?.closest('input, textarea, [contenteditable="true"]');
            if (isFromEditable) {
                return;
            }
            if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                solidProps1.onSelect?.(solidProps1.session.id);
            }
        }} class={`block w-full text-left px-3 py-2 rounded-lg transition-smooth group relative touch-tap cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 ${solidProps1.isActive ? 'bg-primary-50/50 dark:bg-primary-900/30 text-primary-700 dark:text-primary-400 border-l-4 border-primary-400 dark:border-primary-600'
            : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 active:bg-gray-200 dark:active:bg-gray-700 border-l-4 border-transparent'} ${solidProps1.className}`} aria-current={solidProps1.isActive ? 'page' : undefined} data-session-item data-testid={`${solidProps1.testIdPrefix}-session-${solidProps1.session.id}`} {...solidState3.handlers}>
            <div class="flex items-center justify-between gap-2">
              <div class="flex items-start gap-2 min-w-0 flex-1">
                {isGroupOrShared() && (<UsersThree size={14} weight="duotone" className="text-primary-500 dark:text-primary-400 mt-1 flex-shrink-0"/>)}
                <div class="flex flex-col min-w-0 flex-1">
                  <EditableText ref={element => editableTitleRef.current = element ?? null} value={displayTitle()} onSave={(newTitle) => solidProps1.onRename?.(newTitle)} isLoading={isTitleGenerating()}/>
                  <span class="text-[11px] text-gray-400 dark:text-gray-500 truncate mt-0.5">
                    {metaParts().join(' • ')}
                    {isGroupOrShared() && solidProps1.session.memberCount && solidProps1.session.memberCount > 0 && (<span class="inline-flex items-center ml-1 rounded-full bg-primary-50 dark:bg-primary-900/30 px-1.5 text-[10px] font-medium text-primary-600 dark:text-primary-400">
                        {solidProps1.session.memberCount}
                      </span>)}
                  </span>
                </div>
              </div>
              {!isTouch() && hasContextMenu() && (<Menu>
                  <MenuButton onClick={(e: MouseEvent) => {
                e.preventDefault();
                e.stopPropagation();
            }} className="opacity-0 group-hover:opacity-100 p-1.5 rounded hover:bg-gray-200 dark:hover:bg-gray-700 transition-smooth flex-shrink-0 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50" aria-label={solidState2.t('BiChat.Sidebar.ChatOptions')} data-testid={`${solidProps1.testIdPrefix}-session-options-${solidProps1.session.id}`}>
                    <DotsThree size={16} className="w-4 h-4" weight="bold"/>
                  </MenuButton>
                  <MenuItems anchor="bottom start" className="w-52 bg-white dark:bg-gray-900 rounded-xl shadow-xl border border-gray-200 dark:border-gray-700 z-30 [--anchor-gap:8px] mt-1 p-2 space-y-1">
                    {solidProps1.mode !== 'archived' && solidProps1.onPin && (<MenuItem>
                        {(solidProps4) => (<button onClick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        solidProps1.onPin?.();
                    }} class={`cursor-pointer flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-gray-700 dark:text-gray-200 transition-smooth ${solidProps4.focus ? 'bg-gray-100 dark:bg-gray-800/70 ring-1 ring-gray-200/80 dark:ring-gray-700/80'
                        : 'hover:bg-gray-50 dark:hover:bg-gray-800'}`} aria-label={solidProps1.session.pinned ? solidState2.t('BiChat.Sidebar.UnpinChat') : solidState2.t('BiChat.Sidebar.PinChat')} data-testid={`${solidProps1.testIdPrefix}-session-pin-${solidProps1.session.id}`}>
                            {solidProps1.session.pinned ? (<Check size={16} className="w-4 h-4"/>) : (<Bookmark size={16} className="w-4 h-4"/>)}
                            {solidProps1.session.pinned ? solidState2.t('BiChat.Sidebar.UnpinChat') : solidState2.t('BiChat.Sidebar.PinChat')}
                          </button>)}
                      </MenuItem>)}
                    {solidProps1.onRename && (<MenuItem>
                        {(solidProps5) => (<button onClick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        editableTitleRef.current?.startEditing();
                        solidProps5.close();
                    }} class={`cursor-pointer flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-gray-700 dark:text-gray-200 transition-smooth ${solidProps5.focus ? 'bg-gray-100 dark:bg-gray-800/70 ring-1 ring-gray-200/80 dark:ring-gray-700/80'
                        : 'hover:bg-gray-50 dark:hover:bg-gray-800'}`} aria-label={solidState2.t('BiChat.Sidebar.RenameChat')} data-testid={`${solidProps1.testIdPrefix}-session-rename-${solidProps1.session.id}`}>
                            <PencilSimple size={16} className="w-4 h-4"/>
                            {solidState2.t('BiChat.Sidebar.RenameChat')}
                          </button>)}
                      </MenuItem>)}
                    {solidProps1.mode !== 'archived' && solidProps1.onRegenerateTitle && (<MenuItem>
                        {(solidProps6) => (<button onClick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        solidProps1.onRegenerateTitle?.();
                        solidProps6.close();
                    }} class={`cursor-pointer flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-gray-700 dark:text-gray-200 transition-smooth ${solidProps6.focus ? 'bg-gray-100 dark:bg-gray-800/70 ring-1 ring-gray-200/80 dark:ring-gray-700/80'
                        : 'hover:bg-gray-50 dark:hover:bg-gray-800'}`} aria-label={solidState2.t('BiChat.Sidebar.RegenerateTitle')} data-testid={`${solidProps1.testIdPrefix}-session-regenerate-${solidProps1.session.id}`}>
                            <ArrowsClockwise size={16} className="w-4 h-4"/>
                            {solidState2.t('BiChat.Sidebar.RegenerateTitle')}
                          </button>)}
                      </MenuItem>)}
                    {solidProps1.mode === 'archived' && solidProps1.onRestore && (<MenuItem>
                        {(solidProps7) => (<button onClick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        solidProps1.onRestore?.();
                    }} class={`cursor-pointer flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-smooth ${solidProps7.focus ? 'text-green-700 dark:text-green-300 bg-green-50 dark:bg-green-900/20 ring-1 ring-green-200/70 dark:ring-green-500/30'
                        : 'text-green-700 dark:text-green-300 hover:bg-green-50/70 dark:hover:bg-green-900/10'}`} aria-label={solidState2.t('BiChat.Archived.RestoreButton')} data-testid={`${solidProps1.testIdPrefix}-session-restore-${solidProps1.session.id}`}>
                            <ArrowUUpLeft size={16} className="w-4 h-4"/>
                            {solidState2.t('BiChat.Archived.RestoreButton')}
                          </button>)}
                      </MenuItem>)}
                    {solidProps1.mode !== 'archived' && solidProps1.onArchive && (<MenuItem>
                        {(solidProps8) => (<button onClick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        solidProps1.onArchive?.();
                    }} class={`cursor-pointer flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-smooth ${solidProps8.focus ? 'text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 ring-1 ring-amber-200/70 dark:ring-amber-500/30'
                        : 'text-amber-600 dark:text-amber-400 hover:bg-amber-50/70 dark:hover:bg-amber-900/10'}`} aria-label={solidState2.t('BiChat.Sidebar.ArchiveChat')} data-testid={`${solidProps1.testIdPrefix}-session-archive-${solidProps1.session.id}`}>
                            <Archive size={16} className="w-4 h-4"/>
                            {solidState2.t('BiChat.Sidebar.ArchiveChat')}
                          </button>)}
                      </MenuItem>)}
                    {solidProps1.onDelete && (<MenuItem>
                        {(solidProps9) => (<button onClick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        solidProps1.onDelete?.();
                    }} class={`cursor-pointer flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-smooth ${solidProps9.focus ? 'text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 ring-1 ring-red-200/70 dark:ring-red-500/30'
                        : 'text-red-600 dark:text-red-400 hover:bg-red-50/70 dark:hover:bg-red-900/10'}`} aria-label={solidState2.t('BiChat.Sidebar.DeleteChat')} data-testid={`${solidProps1.testIdPrefix}-session-delete-${solidProps1.session.id}`}>
                            <Trash size={16} className="w-4 h-4"/>
                            {solidState2.t('BiChat.Sidebar.DeleteChat')}
                          </button>)}
                      </MenuItem>)}
                  </MenuItems>
                </Menu>)}
            </div>
          </div>
          </div>
        </div>
        <TouchContextMenu items={contextMenuItems()} isOpen={menuOpen()} onClose={() => setMenuOpen(false)} anchorRect={menuAnchor()}/>
      </>);
};
SessionItem; /* Solid components are named by their declarations. */
export default SessionItem;
