import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * TableWithExport Component
 * Wraps markdown tables with an export button that sends a message to export the table
 */
import { TableExportButton } from './TableExportButton';
import { useTranslation } from '../hooks/useTranslation';
interface TableWithExportProps {
    /** The table content to render */
    children: JSX.Element;
    /** Function to send a message (from chat context) */
    sendMessage?: (content: string) => void;
    /** Whether sending is disabled (loading or streaming) */
    disabled?: boolean;
    /** Custom export message to send */
    exportMessage?: string;
    /** Export button label */
    exportLabel?: string;
}
export const TableWithExport = function TableWithExport(solidProps1Input: TableWithExportProps) {
    const solidProps1 = mergeProps({ disabled: false } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const resolvedExportMessage = createMemo(() => solidProps1.exportMessage ?? solidState2.t('BiChat.ExportTableToExcel'));
    const resolvedExportLabel = createMemo(() => solidProps1.exportLabel ?? solidState2.t('BiChat.Export'));
    const handleExport = () => {
        solidProps1.sendMessage?.(resolvedExportMessage());
    };
    return (<>
      <div class="markdown-table-wrapper overflow-x-auto">
        <table class="markdown-table w-full border-collapse">{solidProps1.children}</table>
      </div>
      {solidProps1.sendMessage && (<div class="flex justify-end mt-1">
          <TableExportButton onClick={handleExport} disabled={solidProps1.disabled} label={resolvedExportLabel()}/>
        </div>)}
    </>);
};
