import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * TableExportButton Component
 * Small inline button for exporting markdown tables to Excel
 */
import { FileXls } from '../icons';
import { useTranslation } from '../hooks/useTranslation';
interface TableExportButtonProps {
    /** Click handler for export action */
    onClick: () => void;
    /** Whether the button should be disabled */
    disabled?: boolean;
    /** Export button label (defaults to "Export") */
    label?: string;
    /** Disabled tooltip text */
    disabledTooltip?: string;
}
export const TableExportButton = function TableExportButton(solidProps1Input: TableExportButtonProps) {
    const solidProps1 = mergeProps({ disabled: false } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const resolvedLabel = createMemo(() => solidProps1.label ?? solidState2.t('BiChat.Export'));
    const resolvedDisabledTooltip = createMemo(() => solidProps1.disabledTooltip ?? solidState2.t('BiChat.Common.PleaseWait'));
    return (<button type="button" onClick={solidProps1.onClick} disabled={solidProps1.disabled} class="cursor-pointer inline-flex items-center gap-1 px-2 py-1 text-xs font-medium text-green-700 dark:text-green-400 hover:text-green-800 dark:hover:text-green-300 disabled:text-gray-400 disabled:cursor-not-allowed transition-colors rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50" aria-label={resolvedLabel()} title={solidProps1.disabled ? resolvedDisabledTooltip() : resolvedLabel()}>
      <FileXls size={16} weight="fill"/>
      <span>{resolvedLabel()}</span>
    </button>);
};
