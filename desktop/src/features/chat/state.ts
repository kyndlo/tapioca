import type { ChatMessage, ChatStreamEvent, ChatToolStep } from "./types";

export interface ChatDraft {
  content: string;
  thinking: string;
  tools: ChatToolStep[];
  complete: boolean;
  error?: string;
}

export const emptyChatDraft = (): ChatDraft => ({
  content: "",
  thinking: "",
  tools: [],
  complete: false,
});

export function applyStreamEvent(
  draft: ChatDraft,
  event: ChatStreamEvent,
): ChatDraft {
  switch (event.type) {
    case "thinking.delta":
      return { ...draft, thinking: draft.thinking + event.text };
    case "content.delta":
      return { ...draft, content: draft.content + event.text };
    case "tool": {
      const index = draft.tools.findIndex((tool) => tool.id === event.tool.id);
      const tools = [...draft.tools];
      if (index === -1) tools.push(event.tool);
      else tools[index] = event.tool;
      return { ...draft, tools };
    }
    case "completed":
      return { ...draft, complete: true };
    case "error":
      return { ...draft, complete: true, error: event.message };
  }
}

export function isByeCommand(value: string): boolean {
  return value.trim().toLowerCase() === "/bye";
}

export function lastUserPrompt(messages: ChatMessage[]): string | undefined {
  return [...messages].reverse().find((message) => message.role === "user")
    ?.content;
}

export function chatMessagesForRequest(messages: ChatMessage[]) {
  return messages.map(({ role, content }) => ({ role, content }));
}
