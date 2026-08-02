import { describe, expect, it } from "vitest";
import {
  applyStreamEvent,
  chatMessagesForRequest,
  emptyChatDraft,
  isByeCommand,
  lastUserPrompt,
} from "./state";

describe("chat feature state", () => {
  it("recognizes only the exact case-insensitive /bye command", () => {
    expect(isByeCommand(" /BYE ")).toBe(true);
    expect(isByeCommand("bye")).toBe(false);
    expect(isByeCommand("/bye now")).toBe(false);
  });

  it("accumulates thinking, answer, and updated tool steps", () => {
    let draft = emptyChatDraft();
    draft = applyStreamEvent(draft, { type: "thinking.delta", text: "Check " });
    draft = applyStreamEvent(draft, { type: "thinking.delta", text: "files." });
    draft = applyStreamEvent(draft, {
      type: "tool",
      tool: { id: "tool-1", name: "Read files", status: "running" },
    });
    draft = applyStreamEvent(draft, {
      type: "tool",
      tool: {
        id: "tool-1",
        name: "Read files",
        status: "completed",
        summary: "3 files",
      },
    });
    draft = applyStreamEvent(draft, { type: "content.delta", text: "Done." });
    draft = applyStreamEvent(draft, { type: "completed" });
    expect(draft).toMatchObject({
      thinking: "Check files.",
      content: "Done.",
      complete: true,
    });
    expect(draft.tools).toEqual([
      expect.objectContaining({ id: "tool-1", status: "completed" }),
    ]);
  });

  it("prepares safe role/content history and finds regeneration prompt", () => {
    const messages = [
      {
        id: "1",
        role: "user" as const,
        content: "First",
        createdAt: "2026-08-01T00:00:00Z",
      },
      {
        id: "2",
        role: "assistant" as const,
        content: "Answer",
        thinking: "private working",
        createdAt: "2026-08-01T00:00:01Z",
      },
      {
        id: "3",
        role: "user" as const,
        content: "Latest",
        createdAt: "2026-08-01T00:00:02Z",
      },
    ];
    expect(lastUserPrompt(messages)).toBe("Latest");
    expect(chatMessagesForRequest(messages)).toEqual([
      { role: "user", content: "First" },
      { role: "assistant", content: "Answer" },
      { role: "user", content: "Latest" },
    ]);
  });
});
