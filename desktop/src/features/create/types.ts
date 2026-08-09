export const creatorModes = ["image", "video", "speech", "voice-clone"] as const;
export type CreatorMode = (typeof creatorModes)[number];

export interface CreatorModel {
  id: string;
  name: string;
  modes: CreatorMode[];
  ready: boolean;
  detail?: string;
  limits?: {
    maxWidth?: number;
    maxHeight?: number;
    maxFrames?: number;
  };
  supportsInputImage?: boolean;
  supportsLoRA?: boolean;
  requiresInputImage?: boolean;
  requiresVoiceReference?: boolean;
  defaults?: Partial<CreatorAdvancedSettings>;
}

export type LocalFileKind = "image" | "audio" | "lora";

/** Opaque token selected by trusted main-process code; never a user-entered path. */
export interface LocalFileSelection {
  token: string;
  name: string;
  kind: LocalFileKind;
  previewUrl?: string;
}

export interface HfLoraSource {
  type: "huggingface";
  reference: string;
}

export interface LocalLoraSource {
  type: "local";
  file: LocalFileSelection;
}

export interface CreatorLora {
  id: string;
  source: HfLoraSource | LocalLoraSource;
  weight: number;
}

export interface CreatorLoraOption {
  reference: string;
  name: string;
  bytes: number;
  compatible: boolean;
  reason?: string;
}

export interface CreatorAdvancedSettings {
  width: number;
  height: number;
  steps: number;
  seed?: number;
  frames: number;
  fps: number;
}

export interface CreatorRequest {
  mode: CreatorMode;
  modelId: string;
  prompt: string;
  text?: string;
  inputImage?: LocalFileSelection;
  voiceReference?: LocalFileSelection;
  loras: CreatorLora[];
  settings: CreatorAdvancedSettings;
}

export type CreatorProgressEvent =
  | { type: "queued"; message?: string }
  | { type: "progress"; progress: number; message?: string }
  | { type: "preview"; previewUrl: string }
  | { type: "completed"; output: CreatorOutput }
  | { type: "error"; message: string; retryable: boolean };

export interface CreatorOutput {
  id: string;
  mode: CreatorMode;
  mediaType: "image" | "video" | "audio";
  url: string;
  createdAt: string;
  modelName: string;
  prompt?: string;
  metadata: Record<string, string | number | boolean>;
}

export interface CreatorAdapter {
  models(mode: CreatorMode): Promise<CreatorModel[]>;
  availableLoras(modelId: string): Promise<CreatorLoraOption[]>;
  outputs(): Promise<CreatorOutput[]>;
  pickFile(kind: LocalFileKind): Promise<LocalFileSelection | undefined>;
  saveVoiceRecording(bytes: Uint8Array, durationSeconds: number): Promise<LocalFileSelection>;
  generate(
    request: CreatorRequest,
    onEvent: (event: CreatorProgressEvent) => void,
  ): Promise<{ jobId: string }>;
  cancel(jobId: string): Promise<void>;
  reveal(outputId: string): Promise<void>;
  saveMetadata(outputId: string): Promise<void>;
}
