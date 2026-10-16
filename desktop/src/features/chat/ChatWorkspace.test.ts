import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ChatWorkspace } from "./ChatWorkspace";
import type { ChatAdapter } from "./types";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT =
  true;

let container: HTMLDivElement | undefined;
let root: ReturnType<typeof createRoot> | undefined;

afterEach(async () => {
  if (root) await act(() => root?.unmount());
  container?.remove();
  root = undefined;
  container = undefined;
});

describe("ChatWorkspace", () => {
  it("allows composing while the sidecar is online and the model server is stopped", async () => {
    Element.prototype.scrollIntoView = vi.fn();
    const adapter: ChatAdapter = {
      connection: vi.fn().mockResolvedValue("startable"),
      reconnect: vi.fn().mockResolvedValue("startable"),
      models: vi.fn().mockResolvedValue([
        { id: "gemma", name: "Gemma", ready: true },
      ]),
      sessions: vi.fn().mockResolvedValue([]),
      loadSession: vi.fn(),
      send: vi.fn().mockResolvedValue({
        sessionId: "session",
        assistantMessageId: "assistant",
      }),
      stop: vi.fn(),
    };
    container = document.createElement("div");
    document.body.append(container);
    root = createRoot(container);
    await act(async () => {
      root?.render(createElement(ChatWorkspace, { adapter }));
    });
    const input = container.querySelector<HTMLTextAreaElement>("#chat-prompt");
    expect(input?.disabled).toBe(false);
    expect(container.textContent).toContain(
      "The selected model server will start when you send.",
    );
  });

  it("/bye awaits adapter shutdown and closes the conversation", async () => {
    Element.prototype.scrollIntoView = vi.fn();
    const stop = vi.fn().mockResolvedValue(undefined);
    const adapter: ChatAdapter = {
      connection: vi.fn().mockResolvedValue("startable"),
      reconnect: vi.fn().mockResolvedValue("startable"),
      models: vi.fn().mockResolvedValue([{ id: "gemma", name: "Gemma", ready: true }]),
      sessions: vi.fn().mockResolvedValue([]),
      loadSession: vi.fn(),
      send: vi.fn(),
      stop,
    };
    container = document.createElement("div");
    document.body.append(container);
    root = createRoot(container);
    await act(async () => root?.render(createElement(ChatWorkspace, { adapter })));
    const input = container.querySelector<HTMLTextAreaElement>("#chat-prompt");
    await act(async () => {
      if (!input) return;
      const setter = Object.getOwnPropertyDescriptor(
        HTMLTextAreaElement.prototype,
        "value",
      )?.set;
      setter?.call(input, "/bye");
      input.dispatchEvent(new Event("input", { bubbles: true }));
    });
    const send = Array.from(container.querySelectorAll("button")).find(
      (button) => button.textContent === "Send",
    );
    await act(async () => send?.click());
    expect(stop).toHaveBeenCalledOnce();
    expect(container.textContent).toContain("Conversation ended with /bye.");
  });
});

it("sends a selected attachment with the prompt and keeps its transcript preview", async () => {
  Element.prototype.scrollIntoView = vi.fn();
  class DecodedImage {
    width = 512;
    height = 256;
    onload?: () => void;
    set src(_value: string) { queueMicrotask(() => this.onload?.()); }
  }
  vi.stubGlobal("Image", DecodedImage);
  const send = vi.fn().mockResolvedValue({sessionId: "session", assistantMessageId: "answer"});
  const adapter: ChatAdapter = {
    connection: vi.fn().mockResolvedValue("ready"),
    reconnect: vi.fn().mockResolvedValue("ready"),
    models: vi.fn().mockResolvedValue([{id: "minicpm", name: "MiniCPM", ready: true}]),
    sessions: vi.fn().mockResolvedValue([]),
    loadSession: vi.fn(), send, stop: vi.fn(),
  };
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
  try {
    await act(async () => root?.render(createElement(ChatWorkspace, {adapter})));
    const picker = container.querySelector<HTMLInputElement>('input[type="file"]');
    const file = new File(["fixture"], "scene.png", {type: "image/png"});
    Object.defineProperty(picker, "files", {value: [file]});
    await act(async () => {
      picker?.dispatchEvent(new Event("change", {bubbles: true}));
      await new Promise((resolve) => setTimeout(resolve, 20));
    });
    expect(container.querySelector('img[alt="Pending image 1"]')).not.toBeNull();
    const input = container.querySelector<HTMLTextAreaElement>("#chat-prompt");
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")?.set?.call(input,"Describe");
      input?.dispatchEvent(new Event("input", {bubbles: true}));
    });
    await act(async () => container?.querySelector<HTMLButtonElement>('button[type="submit"]')?.click());
    expect(send).toHaveBeenCalledWith(expect.objectContaining({messages: [expect.objectContaining({content: "Describe", images: ["data:image/png;base64,Zml4dHVyZQ=="]})]}), expect.any(Function));
    expect(container.querySelector('img[alt="Attached image 1"]')).not.toBeNull();
    expect(container.querySelector('img[alt="Pending image 1"]')).toBeNull();
  } finally {
    vi.unstubAllGlobals();
  }
});
