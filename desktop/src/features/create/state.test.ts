import { describe, expect, it } from "vitest";
import {
  defaultCreatorSettings,
  moveLora,
  parseHfLoraReference,
  validateCreatorRequest,
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
});
