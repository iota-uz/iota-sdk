import type { AssistantTurnLifecycle, ConversationTurn, PendingQuestion, StreamInterruptPayload, } from "../types";
import { MessageRole } from "../types";
import { isPlaceholderWaitingAssistantTurn } from "../utils/assistantTurnState";
export function normalizeQuestionType(rawType: unknown): "SINGLE_CHOICE" | "MULTIPLE_CHOICE" {
    const normalized = String(rawType || "")
        .trim()
        .toUpperCase()
        .replace(/[\s-]+/g, "_");
    return normalized === "MULTIPLE_CHOICE" ? "MULTIPLE_CHOICE" : "SINGLE_CHOICE";
}
export function isOpenQuestionStatus(status: PendingQuestion["status"] | undefined): boolean {
    return (status === "PENDING" ||
        status === "ANSWER_SUBMITTED" ||
        status === "REJECT_SUBMITTED" ||
        status === "ANSWER_RESUME_FAILED" ||
        status === "REJECT_RESUME_FAILED");
}
export function shouldPromptForHumanInput(status: PendingQuestion["status"] | undefined): boolean {
    return (status === "PENDING" ||
        status === "ANSWER_RESUME_FAILED" ||
        status === "REJECT_RESUME_FAILED");
}
export function pendingQuestionFromInterrupt(interrupt: StreamInterruptPayload | undefined, fallbackTurnId: string): PendingQuestion | null {
    if (!interrupt) {
        return null;
    }
    const checkpointId = interrupt.checkpointId?.trim();
    if (!checkpointId) {
        return null;
    }
    const questions = Array.isArray(interrupt.questions)
        ? interrupt.questions
            .filter((question) => !!question && typeof question.id === "string")
            .map((question) => ({
            id: question.id,
            text: typeof question.text === "string" ? question.text : "",
            type: normalizeQuestionType(question.type),
            options: Array.isArray(question.options)
                ? question.options
                    .filter((option) => !!option && typeof option.id === "string")
                    .map((option) => ({
                    id: option.id,
                    label: typeof option.label === "string" ? option.label : "",
                    value: option.id,
                }))
                : [],
        }))
        : [];
    return {
        id: checkpointId,
        turnId: fallbackTurnId,
        agentName: interrupt.agentName,
        questions,
        status: "PENDING",
    };
}
export function resolvePendingQuestionTurnIndex(turns: ConversationTurn[], pendingQuestion: PendingQuestion | null): number {
    if (!pendingQuestion ||
        !isOpenQuestionStatus(pendingQuestion.status) ||
        turns.length === 0) {
        return -1;
    }
    const pendingTurnId = pendingQuestion.turnId?.trim();
    if (pendingTurnId) {
        const explicitMatch = turns.findIndex((turn) => turn.id === pendingTurnId ||
            turn.userTurn.id === pendingTurnId ||
            turn.assistantTurn?.id === pendingTurnId);
        if (explicitMatch !== -1) {
            return explicitMatch;
        }
    }
    for (let i = turns.length - 1; i >= 0; i--) {
        if (turns[i].assistantTurn) {
            return i;
        }
    }
    return turns.length - 1;
}
export function applyTurnLifecycleForPendingQuestion(turns: ConversationTurn[], pendingQuestion: PendingQuestion | null): ConversationTurn[] {
    const pendingIndex = resolvePendingQuestionTurnIndex(turns, pendingQuestion);
    if (turns.length === 0) {
        return turns;
    }
    let changed = false;
    const nextTurns = turns.map((turn, index) => {
        const shouldWaitForInput = pendingIndex === index;
        const desiredLifecycle: AssistantTurnLifecycle = shouldWaitForInput
            ? shouldPromptForHumanInput(pendingQuestion?.status)
                ? "waiting_for_human_input"
                : "complete"
            : "complete";
        if (!turn.assistantTurn) {
            if (!shouldWaitForInput || !pendingQuestion) {
                return turn;
            }
            changed = true;
            return {
                ...turn,
                assistantTurn: {
                    id: `${pendingQuestion.id}-assistant`,
                    role: MessageRole.Assistant,
                    content: "",
                    citations: [],
                    artifacts: [],
                    codeOutputs: [],
                    lifecycle: desiredLifecycle,
                    createdAt: turn.createdAt,
                },
            };
        }
        if (!shouldWaitForInput &&
            isPlaceholderWaitingAssistantTurn(turn.assistantTurn)) {
            changed = true;
            return {
                ...turn,
                assistantTurn: undefined,
            };
        }
        if (turn.assistantTurn.lifecycle === desiredLifecycle) {
            return turn;
        }
        changed = true;
        return {
            ...turn,
            assistantTurn: {
                ...turn.assistantTurn,
                lifecycle: desiredLifecycle,
            },
        };
    });
    return changed ? nextTurns : turns;
}
