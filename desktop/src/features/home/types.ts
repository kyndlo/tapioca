export type ReadinessState = "complete" | "current" | "pending" | "blocked";

export interface ReadinessItem {
  id: string;
  title: string;
  description: string;
  state: ReadinessState;
  actionLabel?: string;
  destination?: string;
}

export interface HardwareSummary {
  platform: string;
  processor: string;
  memoryBytes: number;
  accelerator?: string;
}

export interface StorageSummary {
  modelsBytes: number;
  availableBytes: number;
  location: string;
}

export type ActivityKind =
  | "model"
  | "chat"
  | "image"
  | "video"
  | "voice"
  | "agent";

export interface RecentActivity {
  id: string;
  kind: ActivityKind;
  title: string;
  detail: string;
  occurredAt: string;
  destination?: string;
}

export interface HomeSnapshot {
  readiness: ReadinessItem[];
  hardware: HardwareSummary;
  storage: StorageSummary;
  recentActivity: RecentActivity[];
}

export interface HomeAdapter {
  getSnapshot(signal?: AbortSignal): Promise<HomeSnapshot>;
}

export interface HomeNavigation {
  open(destination: string): void;
}
