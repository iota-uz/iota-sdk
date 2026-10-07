/**
 * Pure helper functions for the ChatContext provider.
 * Extracted to keep the main provider file focused on state and handlers.
 */
import { MessageRole, type ConversationTurn, type Attachment, type DebugLimits, } from '../types';
/** Tool names that produce persisted artifacts; must match server ArtifactHandler. */
export const ARTIFACT_TOOL_NAMES = new Set([
    'code_interpreter',
    'draw_chart',
    'export_query_to_excel',
    'export_data_to_excel',
    'export_to_pdf',
]);
export function generateTempId(prefix: string): string {
    return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
}
export function createPendingTurn(sessionId: string, content: string, attachments: Attachment[] = []): ConversationTurn {
    const now = new Date().toISOString();
    return {
        id: generateTempId('turn'),
        sessionId,
        userTurn: {
            id: generateTempId('user'),
            content,
            attachments,
            createdAt: now,
        },
        createdAt: now,
    };
}
export function createCompactedSystemTurn(sessionId: string, summary: string): ConversationTurn {
    const now = new Date().toISOString();
    return {
        id: generateTempId('turn'),
        sessionId,
        userTurn: {
            id: generateTempId('user'),
            content: '',
            attachments: [],
            createdAt: now,
        },
        assistantTurn: {
            id: generateTempId('assistant'),
            role: MessageRole.System,
            content: summary,
            citations: [],
            artifacts: [],
            codeOutputs: [],
            lifecycle: 'complete',
            createdAt: now,
        },
        createdAt: now,
    };
}
export type SlashCommandName = '/clear' | '/debug' | '/compact';
export interface ParsedSlashCommand {
    name: SlashCommandName;
    hasArgs: boolean;
}
export function parseSlashCommand(input: string): ParsedSlashCommand | null {
    const trimmed = input.trim();
    if (!trimmed.startsWith('/')) {
        return null;
    }
    const parts = trimmed.split(/\s+/).filter(Boolean);
    if (parts.length === 0) {
        return null;
    }
    const candidate = parts[0].toLowerCase();
    if (candidate !== '/clear' && candidate !== '/debug' && candidate !== '/compact') {
        return null;
    }
    return {
        name: candidate,
        hasArgs: parts.length > 1,
    };
}
export function readDebugLimitsFromGlobalContext(): DebugLimits | null {
    if (typeof window === 'undefined') {
        return null;
    }
    const limits = window.__APPLET_CONTEXT__?.extensions?.debug?.limits;
    if (!limits) {
        return null;
    }
    const { policyMaxTokens, modelMaxTokens, effectiveMaxTokens, completionReserveTokens, } = limits;
    if (typeof policyMaxTokens !== 'number' ||
        typeof modelMaxTokens !== 'number' ||
        typeof effectiveMaxTokens !== 'number' ||
        typeof completionReserveTokens !== 'number') {
        return null;
    }
    return {
        policyMaxTokens,
        modelMaxTokens,
        effectiveMaxTokens,
        completionReserveTokens,
    };
}
export function readReasoningEffortOptionsFromGlobalContext(): string[] | undefined {
    if (typeof window === 'undefined') {
        return undefined;
    }
    const opts = window.__APPLET_CONTEXT__?.extensions?.llm?.reasoningEffortOptions;
    if (!Array.isArray(opts) || opts.length === 0) {
        return undefined;
    }
    const filtered = opts.filter((o: unknown): o is string => typeof o === 'string');
    if (filtered.length === 0) {
        return undefined;
    }
    return filtered;
}
