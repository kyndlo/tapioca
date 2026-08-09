import { createElement } from "react";
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

  it("locks a route to its explicit operation and does not offer local LoRA files", () => {
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
    expect(markup).not.toContain("Choose local");
  });
});
