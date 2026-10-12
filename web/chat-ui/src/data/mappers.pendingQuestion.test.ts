import { describe, expect, it } from "vitest";
import { sanitizePendingQuestion } from "./mappers";

describe("sanitizePendingQuestion", () => {
  it("keeps option ids as answer values", () => {
    const pending = sanitizePendingQuestion(
      {
        checkpointId: "cp-1",
        turnId: "turn-1",
        agentName: "ali",
        status: "PENDING",
        questions: [
          {
            id: "scope",
            text: "Scope?",
            type: "single_choice",
            options: [
              { id: "sold", label: "Sold only" },
              { id: "all", label: "All policies" },
            ],
          },
        ],
      },
      "session-1",
    );

    expect(pending).not.toBeNull();
    expect(pending?.questions[0]?.options?.[0]?.value).toBe("sold");
    expect(pending?.questions[0]?.options?.[1]?.value).toBe("all");
  });

  it("preserves submitted and failed statuses from the backend payload", () => {
    const submitted = sanitizePendingQuestion(
      {
        checkpointId: "cp-2",
        turnId: "turn-2",
        questions: [],
        status: "ANSWER_SUBMITTED",
      },
      "session-1",
    );
    const failed = sanitizePendingQuestion(
      {
        checkpointId: "cp-3",
        turnId: "turn-3",
        questions: [],
        status: "REJECT_RESUME_FAILED",
      },
      "session-1",
    );

    expect(submitted?.status).toBe("ANSWER_SUBMITTED");
    expect(failed?.status).toBe("REJECT_RESUME_FAILED");
  });
});
