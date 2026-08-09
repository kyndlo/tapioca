import { describe, expect, it } from "vitest";
import {
  defaultCreatorSettings,
  moveLora,
  parseHfLoraReference,
  validateCreatorRequest,
  videoDurationSeconds,
  videoFramesForDuration,
} from "./state";

describe("creator state", () => {
  it("parses beginner-friendly Hugging Face LoRA references", () => {
    expect(parseHfLoraReference("hf://creator/cinematic-motion@0.8")).toEqual({
      reference: "hf://creator/cinematic-motion",
      weight: 0.8,
    });
    expect(parseHfLoraReference("hf://creator/repo#adapter.safetensors")).toEqual({
      reference: "hf://creator/repo#adapter.safetensors",
      weight: 1,
    });
    expect(parseHfLoraReference("https://example.com/model")).toHaveProperty("error");
    expect(parseHfLoraReference("hf://creator/repo@3")).toHaveProperty("error");
  });

  it("reorders a LoRA stack without mutating it", () => {
    const stack = [
      { id: "a", weight: 1, source: { type: "huggingface" as const, reference: "hf://a/repo" } },
      { id: "b", weight: 0.5, source: { type: "huggingface" as const, reference: "hf://b/repo" } },
    ];
    expect(moveLora(stack, "b", -1).map(({ id }) => id)).toEqual(["b", "a"]);
    expect(stack.map(({ id }) => id)).toEqual(["a", "b"]);
  });

  it("validates required mode inputs and safe bounds", () => {
    expect(
      validateCreatorRequest({
        mode: "voice-clone",
        modelId: "",
        prompt: "",
        text: "",
        loras: [],
        settings: { ...defaultCreatorSettings(), width: 255 },
      }),
    ).toEqual(
      expect.arrayContaining([
        "Choose a compatible model.",
        "Enter text to speak.",
        "Choose a voice reference recording.",
        "Width must be 256–2048 and divisible by 8.",
      ]),
    );
  });

  it("turns friendly video durations into model-valid frame counts", () => {
    expect(videoFramesForDuration("minimax-h3:fl2va-int8-mac", 3, 24)).toBe(73);
    expect(videoFramesForDuration("minimax-h3:fl2va-int8-cuda", 5, 24)).toBe(124);
    expect(videoFramesForDuration("ltx-video:2b-fp16", 3, 24)).toBe(73);
    expect(videoFramesForDuration("wan:small", 3, 24)).toBe(73);
    expect(videoDurationSeconds(124, 24)).toBeCloseTo(5.17, 2);
  });

  it("explains invalid expert video values", () => {
    const errors = validateCreatorRequest({
      mode: "video",
      modelId: "minimax-h3:fl2va-int8-mac",
      prompt: "A camera move",
      loras: [],
      settings: { ...defaultCreatorSettings(), width: 510, frames: 120 },
    });
    expect(errors).toEqual(expect.arrayContaining([
      "Width must be 256–2048 and divisible by 32.",
      "MiniMax-H3 duration must resolve to a 17n+5 frame count. Use a duration preset.",
    ]));
  });
});
