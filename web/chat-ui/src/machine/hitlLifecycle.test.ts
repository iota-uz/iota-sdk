import { describe, expect, it } from "vitest";
import { MessageRole } from "../types";
import type {
  ConversationTurn,
  PendingQuestion,
  StreamInterruptPayload,
} from "../types";
import {
  applyTurnLifecycleForPendingQuestion,
  isOpenQuestionStatus,
  normalizeQuestionType,
  pendingQuestionFromInterrupt,
  resolvePendingQuestionTurnIndex,
  shouldPromptForHumanInput,
} from "./hitlLifecycle";

function makeTurn(overrides: Partial<ConversationTurn> = {}): ConversationTurn {
  const createdAt = overrides.createdAt ?? "2026-01-01T00:00:00.000Z";
  return {
    id: overrides.id ?? "turn-1",
    sessionId: overrides.sessionId ?? "session-1",
    userTurn: overrides.userTurn ?? {
      id: "user-1",
      content: "hello",
      attachments: [],
      createdAt,
    },
    assistantTurn: overrides.assistantTurn,
    createdAt,
  };
}

function makeAssistantTurn(
  overrides: Partial<NonNullable<ConversationTurn["assistantTurn"]>> = {},
) {
  return {
    id: overrides.id ?? "assistant-1",
    role: overrides.role ?? MessageRole.Assistant,
    content: overrides.content ?? "answer",
    citations: overrides.citations ?? [],
    artifacts: overrides.artifacts ?? [],
    codeOutputs: overrides.codeOutputs ?? [],
    lifecycle: overrides.lifecycle ?? "complete",
    createdAt: overrides.createdAt ?? "2026-01-01T00:00:00.000Z",
    explanation: overrides.explanation,
    toolCalls: overrides.toolCalls,
    charts: overrides.charts,
    renderTables: overrides.renderTables,
    debug: overrides.debug,
  };
}

function makePendingQuestion(
  overrides: Partial<PendingQuestion> = {},
): PendingQuestion {
  return {
    id: overrides.id ?? "checkpoint-1",
    turnId: overrides.turnId ?? "turn-1",
    questions: overrides.questions ?? [],
    status: overrides.status ?? "PENDING",
    agentName: overrides.agentName,
  };
}

describe("normalizeQuestionType", () => {
  it("normalizes supported variations", () => {
    expect(normalizeQuestionType("MULTIPLE CHOICE")).toBe("MULTIPLE_CHOICE");
    expect(normalizeQuestionType("multiple-choice")).toBe("MULTIPLE_CHOICE");
  });

  it("falls back to SINGLE_CHOICE", () => {
    expect(normalizeQuestionType("anything-else")).toBe("SINGLE_CHOICE");
  });
});

describe("pendingQuestionFromInterrupt", () => {
  it("maps interrupt payload to pending question", () => {
    const interrupt: StreamInterruptPayload = {
      checkpointId: "cp-1",
      agentName: "Analyst",
      questions: [
        {
          id: "q-1",
          text: "Pick one",
          type: "single choice",
          options: [{ id: "o-1", label: "A" }],
        },
      ],
    };

    const pending = pendingQuestionFromInterrupt(interrupt, "turn-fallback");
    expect(pending).not.toBeNull();
    expect(pending?.id).toBe("cp-1");
    expect(pending?.turnId).toBe("turn-fallback");
    expect(pending?.agentName).toBe("Analyst");
    expect(pending?.questions[0]?.type).toBe("SINGLE_CHOICE");
    expect(pending?.questions[0]?.options?.[0]?.value).toBe("o-1");
  });

  it("returns null for malformed checkpointId", () => {
    const pending = pendingQuestionFromInterrupt(
      { checkpointId: "", questions: [] },
      "turn-fallback",
    );
    expect(pending).toBeNull();
  });
});

describe("resolvePendingQuestionTurnIndex", () => {
  it("finds explicit turnId match", () => {
    const turns = [makeTurn({ id: "turn-1" }), makeTurn({ id: "turn-2" })];
    const idx = resolvePendingQuestionTurnIndex(
      turns,
      makePendingQuestion({ turnId: "turn-2" }),
    );
    expect(idx).toBe(1);
  });

  it("falls back to last turn with assistant", () => {
    const turns = [
      makeTurn({ id: "turn-1" }),
      makeTurn({
        id: "turn-2",
        assistantTurn: makeAssistantTurn({ id: "assistant-2" }),
      }),
      makeTurn({
        id: "turn-3",
        assistantTurn: makeAssistantTurn({ id: "assistant-3" }),
      }),
    ];
    const idx = resolvePendingQuestionTurnIndex(
      turns,
      makePendingQuestion({ turnId: "not-found" }),
    );
    expect(idx).toBe(2);
  });

  it("keeps submitted and failed questions anchored to the same turn", () => {
    const turns = [makeTurn({ id: "turn-1" }), makeTurn({ id: "turn-2" })];

    expect(
      resolvePendingQuestionTurnIndex(
        turns,
        makePendingQuestion({
          turnId: "turn-2",
          status: "ANSWER_SUBMITTED",
        }),
      ),
    ).toBe(1);
    expect(
      resolvePendingQuestionTurnIndex(
        turns,
        makePendingQuestion({
          turnId: "turn-2",
          status: "ANSWER_RESUME_FAILED",
        }),
      ),
    ).toBe(1);
  });
});

describe("applyTurnLifecycleForPendingQuestion", () => {
  it("marks matched assistant turn as waiting_for_human_input", () => {
    const turns = [
      makeTurn({
        id: "turn-1",
        assistantTurn: makeAssistantTurn({ id: "assistant-1" }),
      }),
      makeTurn({
        id: "turn-2",
        assistantTurn: makeAssistantTurn({ id: "assistant-2" }),
      }),
    ];

    const updated = applyTurnLifecycleForPendingQuestion(
      turns,
      makePendingQuestion({ turnId: "turn-2" }),
    );

    expect(updated[0].assistantTurn?.lifecycle).toBe("complete");
    expect(updated[1].assistantTurn?.lifecycle).toBe("waiting_for_human_input");
  });

  it("creates placeholder assistant turn when pending turn has none", () => {
    const turns = [makeTurn({ id: "turn-1" })];
    const updated = applyTurnLifecycleForPendingQuestion(
      turns,
      makePendingQuestion({ turnId: "turn-1", id: "checkpoint-9" }),
    );

    expect(updated[0].assistantTurn).toBeDefined();
    expect(updated[0].assistantTurn?.id).toBe("checkpoint-9-assistant");
    expect(updated[0].assistantTurn?.lifecycle).toBe("waiting_for_human_input");
  });

  it("removes empty HITL placeholder assistant turn when pending question is removed", () => {
    const turns = [
      makeTurn({
        id: "turn-1",
        assistantTurn: makeAssistantTurn({
          id: "checkpoint-9-assistant",
          content: "",
          citations: [],
          artifacts: [],
          codeOutputs: [],
          lifecycle: "waiting_for_human_input",
        }),
      }),
    ];

    const updated = applyTurnLifecycleForPendingQuestion(turns, null);
    expect(updated[0].assistantTurn).toBeUndefined();
  });

  it("clears waiting lifecycle when pending question is removed", () => {
    const turns = [
      makeTurn({
        id: "turn-1",
        assistantTurn: makeAssistantTurn({
          lifecycle: "waiting_for_human_input",
        }),
      }),
    ];

    const updated = applyTurnLifecycleForPendingQuestion(turns, null);
    expect(updated[0].assistantTurn?.lifecycle).toBe("complete");
  });

  it("returns original array when no lifecycle changes are needed", () => {
    const turns = [
      makeTurn({
        id: "turn-1",
        assistantTurn: makeAssistantTurn({
          lifecycle: "waiting_for_human_input",
        }),
      }),
    ];

    const updated = applyTurnLifecycleForPendingQuestion(
      turns,
      makePendingQuestion({ turnId: "turn-1" }),
    );

    expect(updated).toBe(turns);
  });

  it("does not leave assistant turns waiting when the question is already resuming", () => {
    const turns = [
      makeTurn({
        id: "turn-1",
        assistantTurn: makeAssistantTurn({
          lifecycle: "waiting_for_human_input",
        }),
      }),
    ];

    const updated = applyTurnLifecycleForPendingQuestion(
      turns,
      makePendingQuestion({ turnId: "turn-1", status: "ANSWER_SUBMITTED" }),
    );

    expect(updated[0].assistantTurn?.lifecycle).toBe("complete");
  });
});

describe("open question helpers", () => {
  it("recognizes open and retryable statuses", () => {
    expect(isOpenQuestionStatus("PENDING")).toBe(true);
    expect(isOpenQuestionStatus("ANSWER_SUBMITTED")).toBe(true);
    expect(isOpenQuestionStatus("REJECT_RESUME_FAILED")).toBe(true);
    expect(isOpenQuestionStatus("ANSWERED")).toBe(false);
  });

  it("only prompts for human input when the user can still act", () => {
    expect(shouldPromptForHumanInput("PENDING")).toBe(true);
    expect(shouldPromptForHumanInput("ANSWER_RESUME_FAILED")).toBe(true);
    expect(shouldPromptForHumanInput("ANSWER_SUBMITTED")).toBe(false);
    expect(shouldPromptForHumanInput("REJECTED")).toBe(false);
  });
});
