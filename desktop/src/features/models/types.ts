export type ModelKind = "chat" | "image" | "video" | "speech";
export type ModelPlatform = "macos" | "windows" | "linux";
export type Accelerator = "apple" | "nvidia" | "amd" | "intel" | "cpu";

export interface ModelRequirements {
  memoryBytes: number;
  recommendedMemoryBytes?: number;
  diskBytes: number;
  platforms: ModelPlatform[];
  accelerators: Accelerator[];
}

export interface ModelRecord {
  id: string;
  name: string;
  creator: string;
  description: string;
  kind: ModelKind;
  backend: string;
  tags: string[];
  requirements: ModelRequirements;
  installed: boolean;
  installedBytes?: number;
	updatedAt?: string;
	gated?: boolean;
	license?: string;
	licenseUrl?: string;
}

export interface MachineProfile {
  platform: ModelPlatform;
  memoryBytes: number;
  availableDiskBytes: number;
  accelerators: Accelerator[];
}

export interface PullProgress {
  receivedBytes: number;
  totalBytes: number;
}

export interface PullOptions {
	signal: AbortSignal;
	acceptLicense?: boolean;
	accessToken?: string;
  onProgress(progress: PullProgress): void;
}

export interface ModelHubAdapter {
  listModels(signal?: AbortSignal): Promise<ModelRecord[]>;
  pullModel(modelId: string, options: PullOptions): Promise<ModelRecord>;
  cancelPull(modelId: string): Promise<void>;
  removeModel(modelId: string): Promise<void>;
}

export type CompatibilityLevel = "compatible" | "tight" | "incompatible";

export interface CompatibilityResult {
  level: CompatibilityLevel;
  reasons: string[];
}
