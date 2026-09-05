import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { CreatorScreen } from "./CreatorScreen";
import type { CreatorAdapter } from "./types";

function mockAdapter(): CreatorAdapter {
  return {
    models: vi.fn().mockResolvedValue([]),
    availableLoras: vi.fn().mockResolvedValue([]),
    outputs: vi.fn().mockResolvedValue([]),
    pickFile: vi.fn().mockResolvedValue(undefined),
    saveVoiceRecording: vi.fn(),
    generate: vi.fn().mockResolvedValue({ jobId: "job-1" }),
    cancel: vi.fn().mockResolvedValue(undefined),
    reveal: vi.fn().mockResolvedValue(undefined),
    saveMetadata: vi.fn().mockResolvedValue(undefined),
  };
}

describe("CreatorScreen", () => {
  it("requires voice permission again when the selected recording changes", async () => {
    (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    const adapter = mockAdapter();
    vi.mocked(adapter.models).mockResolvedValue([{ id: "speech", name: "Speech", modes: ["speech"], ready: true }]);
    vi.mocked(adapter.pickFile).mockResolvedValueOnce({ token: "voice-a", name: "a.wav", kind: "audio" }).mockResolvedValueOnce({ token: "voice-b", name: "b.wav", kind: "audio" });
    const container = document.createElement("div"); document.body.append(container);
    const root = createRoot(container);
    const button = (label: string) => Array.from(container.querySelectorAll("button")).find((item) => item.textContent === label)!;
    try {
      await act(async () => root.render(createElement(CreatorScreen, { adapter, initialMode: "speech", modes: ["speech"] })));
      await act(async () => button("Choose file").click());
      await act(async () => button("Generate").click());
      expect(container.textContent).toContain("Confirm that you have permission");
      expect(adapter.generate).not.toHaveBeenCalled();
      await act(() => container.querySelector<HTMLInputElement>(".creator-voice-consent input")!.click());
      expect(container.querySelector<HTMLInputElement>(".creator-voice-consent input")!.checked).toBe(true);
      await act(async () => button("Change").click());
      expect(container.querySelector<HTMLInputElement>(".creator-voice-consent input")!.checked).toBe(false);
      await act(() => {
        const textarea = container.querySelector("textarea")!;
        Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(textarea, "Hello from my local voice.");
        textarea.dispatchEvent(new Event("input", { bubbles: true }));
      });
      await act(async () => button("Generate").click());
      expect(adapter.generate).not.toHaveBeenCalled();
      await act(() => container.querySelector<HTMLInputElement>(".creator-voice-consent input")!.click());
      await act(async () => button("Generate").click());
      expect(adapter.generate).toHaveBeenCalledWith(expect.objectContaining({ voiceReference: expect.objectContaining({ token: "voice-b" }), text: "Hello from my local voice." }), expect.any(Function));
    } finally {
      await act(() => root.unmount()); container.remove();
    }
  });
  it("renders every backend-neutral mode and local privacy promise", () => {
    const markup = renderToStaticMarkup(
      createElement(CreatorScreen, { adapter: mockAdapter() }),
    );
    for (const label of ["Image", "Video", "Speech / TTS", "Voice Clone"]) {
      expect(markup).toContain(label);
    }
    expect(markup).toContain("Image setup");
    expect(markup).toContain("exact output pixels");
    expect(markup).toContain("Generation quality");
    expect(markup).toContain("Local processing");
    expect(markup).toContain("Your prompts and references stay on this machine.");
    expect(markup).not.toContain("/Users/");
  });

  it("locks a route to its explicit operation and offers managed local LoRA import", () => {
    const markup = renderToStaticMarkup(
      createElement(CreatorScreen, {
        adapter: mockAdapter(),
        initialMode: "video",
        modes: ["video"],
      }),
    );
    expect(markup).toContain("Video");
    expect(markup).toContain("Video setup");
    expect(markup).toContain("Duration");
    expect(markup).toContain("Resolution");
    expect(markup).toContain("Expert overrides");
    expect(markup).not.toContain("Speech / TTS");
    expect(markup).toContain("Import from computer");
  });
});
