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

export function videoDurationSeconds(frames: number, fps: number): number {
  return fps > 0 ? frames / fps : 0;
}

export function videoFramesForDuration(
  modelId: string,
  seconds: number,
  fps: number,
  maxFrames = 513,
): number {
  const { offset, stride } = videoFrameShape(modelId);
  const requested = Math.max(offset, Math.round(seconds * Math.max(1, fps)));
  const index = Math.max(0, Math.round((requested - offset) / stride));
  const maximumIndex = Math.max(0, Math.floor((maxFrames - offset) / stride));
  return offset + Math.min(index, maximumIndex) * stride;
}

export function videoFrameShape(modelId: string): { offset: number; stride: number } {
  const normalized = modelId.toLowerCase();
  if (normalized.includes("minimax-h3")) return { offset: 5, stride: 17 };
  if (normalized.includes("ltx-video")) return { offset: 1, stride: 8 };
  return { offset: 1, stride: 4 };
}

export function parseLoraReference(value: string):
  | { reference: string; weight: number }
  | { error: string } {
  const input = value.trim();
  if (/^https:\/\/(?:www\.)?civitai\.(?:com|red)\/models\//i.test(input)) {
    try {
      const parsed = new URL(input);
      const modelId = parsed.pathname.split("/").filter(Boolean)[1];
      const versionId = parsed.searchParams.get("modelVersionId");
      if (/^\d+$/.test(modelId ?? "") && /^\d+$/.test(versionId ?? "")) {
        return { reference: `civitai://${modelId}/${versionId}`, weight: 1 };
      }
    } catch {
      // The provider-specific validation error below is more useful to users.
    }
    return { error: "Civitai links must include a modelVersionId" };
  }
  const match = input.match(/^(hf|ms|modelscope|civitai|local):\/\/(.+)$/);
  if (!match) {
    return { error: "Start with hf://, civitai://, ms://, or local://" };
  }
  const provider = match[1];
  const body = match[2];
  const at = body.lastIndexOf("@");
  const referenceBody = at > 0 ? body.slice(0, at) : body;
  const weightText = at > 0 ? body.slice(at + 1) : "1";
  const parts = referenceBody.split("#");
  if (parts.length > 2 || (parts[1] && !isSafeLoraFile(parts[1]))) {
    return { error: "The optional file must be a safe relative .safetensors path" };
  }
  const repositoryReference = /^[\w.-]+\/[\w.-]+$/;
  const civitaiReference = /^\d+\/\d+$/;
  const localReference = /^[\w.-]+$/;
  const repository = parts[0];
  const valid = provider === "civitai"
    ? civitaiReference.test(repository)
    : provider === "local"
      ? localReference.test(repository)
      : repositoryReference.test(repository);
  if (!valid) {
    return { error: "Use a provider reference with an optional #file.safetensors suffix" };
  }
  const weight = Number(weightText);
  if (!Number.isFinite(weight) || weight < 0 || weight > 2) {
    return { error: "LoRA weight must be between 0 and 2" };
  }
  return { reference: `${provider}://${referenceBody}`, weight };
}

function isSafeLoraFile(value: string): boolean {
  const segments = value.replaceAll("\\", "/").split("/");
  return !value.startsWith("/") && !value.includes("\\") && !/[\0\r\n]/.test(value) &&
    segments.every((segment) => segment !== "" && segment !== "." && segment !== "..") &&
    value.toLowerCase().endsWith(".safetensors");
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
  const dimensionStep = request.mode === "video" ? 32 : 8;
  if (width < 256 || width > 2048 || width % dimensionStep !== 0) {
    errors.push(`Width must be 256–2048 and divisible by ${dimensionStep}.`);
  }
  if (height < 256 || height > 2048 || height % dimensionStep !== 0) {
    errors.push(`Height must be 256–2048 and divisible by ${dimensionStep}.`);
  }
  if (steps < 1 || steps > 100) errors.push("Steps must be between 1 and 100.");
  if (request.mode === "video" && (frames < 1 || frames > 513)) {
    errors.push("Frames must be between 1 and 513.");
  } else if (request.mode === "video") {
    const { offset, stride } = videoFrameShape(request.modelId);
    if (frames < offset || (frames - offset) % stride !== 0) {
      errors.push(
        request.modelId.toLowerCase().includes("minimax-h3")
          ? "MiniMax-H3 duration must resolve to a 17n+5 frame count. Use a duration preset."
          : `This video model requires a ${stride}n+${offset} frame count. Use a duration preset.`,
      );
    }
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
