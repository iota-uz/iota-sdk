import type { AssistantTurn } from '../types';
export function isEmptyAssistantTurn(turn: AssistantTurn): boolean {
    if (turn.content.trim().length > 0) {
        return false;
    }
    if ((turn.explanation?.trim().length ?? 0) > 0) {
        return false;
    }
    if ((turn.citations?.length ?? 0) > 0) {
        return false;
    }
    if ((turn.toolCalls?.length ?? 0) > 0) {
        return false;
    }
    if ((turn.charts?.length ?? 0) > 0) {
        return false;
    }
    if ((turn.renderTables?.length ?? 0) > 0) {
        return false;
    }
    if ((turn.artifacts?.length ?? 0) > 0) {
        return false;
    }
    if ((turn.codeOutputs?.length ?? 0) > 0) {
        return false;
    }
    if (turn.debug) {
        return false;
    }
    return true;
}
export function isPlaceholderWaitingAssistantTurn(turn: AssistantTurn): boolean {
    return turn.lifecycle === 'waiting_for_human_input' && isEmptyAssistantTurn(turn);
}
export function shouldRenderInlineRetry(turn: AssistantTurn, canRegenerate: boolean): boolean {
    if (!canRegenerate) {
        return false;
    }
    if (turn.lifecycle === 'waiting_for_human_input') {
        return false;
    }
    return isEmptyAssistantTurn(turn);
}
