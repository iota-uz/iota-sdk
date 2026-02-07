import { useEffect, useRef, useState, useCallback, FC, ReactNode, CSSProperties } from 'react';
import { createPortal } from 'react-dom';
import { AnimatePresence, motion } from 'framer-motion';

export interface ContextMenuItem {
  id: string;
  label: string;
  icon?: ReactNode;
  onClick: () => void;
  variant?: 'default' | 'danger';
  disabled?: boolean;
}

interface TouchContextMenuProps {
  items: ContextMenuItem[];
  isOpen: boolean;
  onClose: () => void;
  anchorRect: DOMRect | null;
}

export const TouchContextMenu: FC<TouchContextMenuProps> = ({
  items,
  isOpen,
  onClose,
  anchorRect,
}) => {
  const [focusedIndex, setFocusedIndex] = useState(-1);
  const menuRef = useRef<HTMLDivElement>(null);
  const itemRefs = useRef<(HTMLButtonElement | null)[]>([]);

  const enabledIndices = items.reduce<number[]>((acc, item, i) => {
    if (!item.disabled) acc.push(i);
    return acc;
  }, []);

  const focusItem = useCallback((index: number) => {
    setFocusedIndex(index);
    itemRefs.current[index]?.focus();
  }, []);

  // Auto-focus first enabled item on open
  useEffect(() => {
    if (!isOpen) {
      setFocusedIndex(-1);
      return;
    }

    // Small delay to let the menu render
    const timer = requestAnimationFrame(() => {
      if (enabledIndices.length > 0) {
        focusItem(enabledIndices[0]);
      }
    });
    return () => cancelAnimationFrame(timer);
  }, [isOpen, enabledIndices.length, focusItem]);

  // Keyboard navigation
  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      const currentEnabledPos = enabledIndices.indexOf(focusedIndex);

      switch (e.key) {
        case 'Escape':
          e.preventDefault();
          onClose();
          break;
        case 'ArrowDown': {
          e.preventDefault();
          const nextPos = currentEnabledPos < enabledIndices.length - 1
            ? currentEnabledPos + 1
            : 0;
          focusItem(enabledIndices[nextPos]);
          break;
        }
        case 'ArrowUp': {
          e.preventDefault();
          const prevPos = currentEnabledPos > 0
            ? currentEnabledPos - 1
            : enabledIndices.length - 1;
          focusItem(enabledIndices[prevPos]);
          break;
        }
        case 'Home':
          e.preventDefault();
          if (enabledIndices.length > 0) focusItem(enabledIndices[0]);
          break;
        case 'End':
          e.preventDefault();
          if (enabledIndices.length > 0) focusItem(enabledIndices[enabledIndices.length - 1]);
          break;
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose, focusedIndex, enabledIndices, focusItem]);

  if (!isOpen || !anchorRect) return null;

  const viewportHeight = window.innerHeight;
  const menuEstimatedHeight = items.length * 44 + 16;
  const spaceBelow = viewportHeight - anchorRect.bottom;
  const shouldShowAbove = spaceBelow < menuEstimatedHeight && anchorRect.top > spaceBelow;

  const style: CSSProperties = {
    position: 'fixed',
    left: `${anchorRect.left}px`,
    width: `${anchorRect.width}px`,
    zIndex: 9999,
    ...(shouldShowAbove
      ? { bottom: `${viewportHeight - anchorRect.top}px` }
      : { top: `${anchorRect.bottom}px` }),
  };

  return createPortal(
    <AnimatePresence>
      {isOpen && (
        <>
          <div
            className="fixed inset-0 z-[9998]"
            onClick={onClose}
          />

          <motion.div
            ref={menuRef}
            role="menu"
            aria-label="Context menu"
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.95 }}
            transition={{ duration: 0.15 }}
            style={style}
            className="rounded-xl shadow-xl backdrop-blur bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 overflow-hidden"
          >
            <div className="py-2">
              {items.map((item, index) => (
                <button
                  key={item.id}
                  ref={(el) => { itemRefs.current[index] = el; }}
                  role="menuitem"
                  tabIndex={focusedIndex === index ? 0 : -1}
                  onClick={() => {
                    if (!item.disabled) {
                      item.onClick();
                      onClose();
                    }
                  }}
                  disabled={item.disabled}
                  className={`
                    w-full flex items-center gap-3 px-4 py-2.5 min-h-[44px]
                    text-left text-sm font-medium transition-colors
                    ${item.disabled
                      ? 'opacity-50 cursor-not-allowed'
                      : 'hover:bg-gray-100 dark:hover:bg-gray-700 active:bg-gray-200 dark:active:bg-gray-600'}
                    ${item.variant === 'danger'
                      ? 'text-red-600 dark:text-red-400'
                      : 'text-gray-900 dark:text-gray-100'}
                  `}
                >
                  {item.icon && (
                    <span className="flex-shrink-0 w-5 h-5 flex items-center justify-center">
                      {item.icon}
                    </span>
                  )}
                  <span className="flex-1">{item.label}</span>
                </button>
              ))}
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>,
    document.body
  );
};
