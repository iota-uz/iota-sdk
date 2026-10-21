import { describe, it, expect } from "vitest";
import {
  splitIntoTextBlocks,
  readTextBlockOffsets,
  type AssistantTextBlock,
} from "./textBlocks";

describe("splitIntoTextBlocks", () => {
  it("returns an empty list for empty content", () => {
    expect(splitIntoTextBlocks("")).toEqual([]);
  });

  it("returns a single block when no offsets are supplied", () => {
    const got = splitIntoTextBlocks("Hello world");
    expect(got).toEqual<AssistantTextBlock[]>([{ seq: 0, content: "Hello world" }]);
  });

  it("splits content at each offset and includes the trailing segment", () => {
    const content = "Checking weather. Result: sunny. All done.";
    // Offset 17 = end of "Checking weather.", 32 = end of "… Result: sunny.".
    const got = splitIntoTextBlocks(content, [17, 32]);
    expect(got).toEqual<AssistantTextBlock[]>([
      { seq: 0, content: "Checking weather." },
      { seq: 1, content: " Result: sunny." },
      { seq: 2, content: " All done." },
    ]);
  });

  it("clamps offsets beyond the current content length (stale snapshot case)", () => {
    const got = splitIntoTextBlocks("short", [10]);
    // Clamps to 5 then no trailing remainder — single block covering the full content.
    expect(got).toEqual<AssistantTextBlock[]>([{ seq: 0, content: "short" }]);
  });

  it("sorts offsets and ignores duplicates without crashing", () => {
    const got = splitIntoTextBlocks("abcdef", [4, 4, 2]);
    expect(got).toEqual<AssistantTextBlock[]>([
      { seq: 0, content: "ab" },
      { seq: 1, content: "cd" },
      { seq: 2, content: "ef" },
    ]);
  });

  it("preserves unicode boundaries that fall between surrogate pairs safely", () => {
    // NOTE: this intentionally matches Go's byte-offset semantics on the
    // server — text_block_offsets are byte offsets into the UTF-8
    // accumulated Content. The client uses the same indices into the
    // JS string (UTF-16). Consumers who need grapheme-accurate rendering
    // should defer to markdown rendering rather than expecting this
    // function to fix the boundary.
    const got = splitIntoTextBlocks("hi\u{1F600}bye", [2]);
    expect(got[0].content).toBe("hi");
    expect(got[1].content).toBe("\u{1F600}bye");
  });
});

describe("readTextBlockOffsets", () => {
  it("returns empty array for missing or non-array metadata", () => {
    expect(readTextBlockOffsets()).toEqual([]);
    expect(readTextBlockOffsets(null)).toEqual([]);
    expect(readTextBlockOffsets({})).toEqual([]);
    expect(readTextBlockOffsets({ text_block_offsets: "nope" })).toEqual([]);
  });

  it("extracts numeric entries and drops non-numeric ones", () => {
    const got = readTextBlockOffsets({
      text_block_offsets: [0, 10, "20", NaN, -5, 30, null],
    });
    expect(got).toEqual([0, 10, 30]);
  });
});
