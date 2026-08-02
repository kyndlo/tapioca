import type {
  CreatorAdvancedSettings,
  CreatorLora,
  CreatorMode,
  CreatorRequest,
} from "./types";

export const defaultCreatorSettings = (): CreatorAdvancedSettings => ({
  width: 512,
  height: 512,
  steps: 4,
  frames: 49,
  fps: 24,
});

export function parseHfLoraReference(value: string):
  | { reference: string; weight: number }
  | { error: string } {
  const input = value.trim();
  if (!input.startsWith("hf://")) {
    return { error: "Start a Hugging Face reference with hf://" };
  }
  const body = input.slice(5);
  const at = body.lastIndexOf("@");
  const referenceBody = at > 0 ? body.slice(0, at) : body;
  const weightText = at > 0 ? body.slice(at + 1) : "1";
  if (!/^[\w.-]+\/[\w.-]+(?:#[\w./-]+\.safetensors)?$/.test(referenceBody)) {
    return { error: "Use hf://creator/repository or hf://creator/repository#file.safetensors" };
  }
  const weight = Number(weightText);
  if (!Number.isFinite(weight) || weight < 0 || weight > 2) {
    return { error: "LoRA weight must be between 0 and 2" };
  }
  return { reference: `hf://${referenceBody}`, weight };
}

export function moveLora(
  loras: CreatorLora[],
  id: string,
  direction: -1 | 1,
): CreatorLora[] {
  const index = loras.findIndex((lora) => lora.id === id);
  const target = index + direction;
  if (index < 0 || target < 0 || target >= loras.length) return loras;
  const next = [...loras];
  [next[index], next[target]] = [next[target], next[index]];
  return next;
}

export function validateCreatorRequest(request: CreatorRequest): string[] {
  const errors: string[] = [];
  if (!request.modelId) errors.push("Choose a compatible model.");
  if (request.mode === "speech" || request.mode === "voice-clone") {
    if (!request.text?.trim()) errors.push("Enter text to speak.");
  } else if (!request.prompt.trim()) {
    errors.push("Enter a generation prompt.");
  }
  if (request.mode === "voice-clone" && !request.voiceReference) {
    errors.push("Choose a voice reference recording.");
  }
  const { width, height, steps, frames, fps, seed } = request.settings;
  if (width < 256 || width > 2048 || width % 8 !== 0) {
    errors.push("Width must be 256–2048 and divisible by 8.");
  }
  if (height < 256 || height > 2048 || height % 8 !== 0) {
    errors.push("Height must be 256–2048 and divisible by 8.");
  }
  if (steps < 1 || steps > 100) errors.push("Steps must be between 1 and 100.");
  if (request.mode === "video" && (frames < 1 || frames > 241)) {
    errors.push("Frames must be between 1 and 241.");
  }
  if (request.mode === "video" && (fps < 1 || fps > 60)) {
    errors.push("FPS must be between 1 and 60.");
  }
  if (seed !== undefined && (!Number.isInteger(seed) || seed < 0)) {
    errors.push("Seed must be a positive whole number.");
  }
  if (request.loras.some((lora) => lora.weight < 0 || lora.weight > 2)) {
    errors.push("Every LoRA weight must be between 0 and 2.");
  }
  return errors;
}

export function modeLabel(mode: CreatorMode): string {
  return {
    image: "Image",
    video: "Video",
    speech: "Speech / TTS",
    "voice-clone": "Voice Clone",
  }[mode];
}
