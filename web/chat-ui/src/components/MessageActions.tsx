import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { Copy, ArrowClockwise, PencilSimple } from '../icons';
import { MessageRole } from '../types';
import { useToast } from '../hooks/useToast';
import { useTranslation } from '../hooks/useTranslation';
import LoadingSpinner from './LoadingSpinner';
interface ActionableMessage {
    id: string;
    role: MessageRole;
    content: string;
}
interface MessageActionsProps {
    message: ActionableMessage;
    onCopy: (text: string) => Promise<void>;
    onRegenerate?: (messageId: string) => Promise<void>;
    onEdit?: (message: ActionableMessage) => void;
}
function MessageActions(solidProps1: MessageActionsProps) {
    const [copying, setCopying] = createSignal(false);
    const [regenerating, setRegenerating] = createSignal(false);
    const toast = useToast();
    const solidState2 = useTranslation();
    const isUser = solidProps1.message.role === MessageRole.User;
    const handleCopy = async () => {
        setCopying(true);
        try {
            await solidProps1.onCopy?.(solidProps1.message.content);
            toast.success(solidState2.t('BiChat.Message.CopiedToClipboard'));
        }
        catch {
            toast.error(solidState2.t('BiChat.Message.FailedToCopy'));
        }
        finally {
            setCopying(false);
        }
    };
    const handleRegenerate = async () => {
        if (!solidProps1.onRegenerate) {
            return;
        }
        setRegenerating(true);
        try {
            await solidProps1.onRegenerate?.(solidProps1.message.id);
        }
        finally {
            setRegenerating(false);
        }
    };
    return (<div class="flex items-center gap-2">
      {/* Copy button */}
      <button onClick={handleCopy} disabled={copying()} title={copying() ? solidState2.t('BiChat.Message.Copying') : solidState2.t('BiChat.Message.CopyMessage')} class="cursor-pointer text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 hover:text-gray-700 dark:hover:text-gray-300 rounded-md transition-colors disabled:opacity-50 p-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50" aria-label={copying() ? solidState2.t('BiChat.Message.Copying') : solidState2.t('BiChat.Message.CopyMessage')}>
        {copying() ? (<LoadingSpinner variant="spinner" size="sm"/>) : (<Copy size={16} className="w-4 h-4"/>)}
      </button>

      {/* Regenerate button (AI messages only) */}
      {!isUser && solidProps1.onRegenerate && (<button onClick={handleRegenerate} disabled={regenerating()} title={regenerating() ? solidState2.t('BiChat.Message.Regenerating') : solidState2.t('BiChat.Message.Regenerate')} class="cursor-pointer text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 hover:text-gray-700 dark:hover:text-gray-300 rounded-md transition-colors disabled:opacity-50 p-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50" aria-label={regenerating() ? solidState2.t('BiChat.Message.Regenerating') : solidState2.t('BiChat.Message.Regenerate')}>
          {regenerating() ? (<LoadingSpinner variant="spinner" size="sm"/>) : (<ArrowClockwise size={16} className="w-4 h-4"/>)}
        </button>)}

      {/* Edit button (user messages only) */}
      {isUser && solidProps1.onEdit && (<button onClick={() => solidProps1.onEdit?.(solidProps1.message)} title={solidState2.t('BiChat.Message.EditMessage')} class="cursor-pointer text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 hover:text-gray-700 dark:hover:text-gray-300 rounded-md transition-colors p-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50" aria-label={solidState2.t('BiChat.Message.EditMessage')}>
          <PencilSimple size={16} className="w-4 h-4"/>
        </button>)}
    </div>);
}
export default MessageActions;
export { MessageActions };
export type { ActionableMessage };
