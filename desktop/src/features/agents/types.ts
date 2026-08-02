export const agentKinds = [
  "codex",
  "claude",
  "opencode",
  "openclaw",
  "hermes",
] as const;

export type AgentKind = (typeof agentKinds)[number];
export type AgentRunStatus =
  | "idle"
  | "checking"
  | "launching"
  | "running"
  | "stopping"
  | "completed"
  | "failed";

export interface AgentDefinition {
  id: AgentKind;
  name: string;
  description: string;
  installed: boolean;
}

export interface AgentModel {
  id: string;
  name: string;
  ready: boolean;
  context?: string;
}

export interface AgentReadiness {
  server: "ready" | "starting" | "offline";
  endpoint?: string;
  message?: string;
}

export interface WorkspaceSelection {
  path: string;
  displayName: string;
}

export interface AgentEnvironmentEntry {
  name: string;
  value: string;
  sensitive?: boolean;
}

export interface AgentLaunchRequest {
  agent: AgentKind;
  modelId: string;
  workspace: WorkspaceSelection;
}

export type AgentRunEvent =
  | { type: "status"; status: AgentRunStatus; message?: string }
  | { type: "log"; level: "info" | "warn" | "error"; message: string }
  | { type: "exit"; code: number | null };

export interface AgentAdapter {
  definitions(): Promise<AgentDefinition[]>;
  models(): Promise<AgentModel[]>;
  readiness(): Promise<AgentReadiness>;
  chooseWorkspace(): Promise<WorkspaceSelection | undefined>;
  environment(request: AgentLaunchRequest): Promise<AgentEnvironmentEntry[]>;
  launch(
    request: AgentLaunchRequest,
    onEvent: (event: AgentRunEvent) => void,
  ): Promise<{ runId: string }>;
  stop(runId: string): Promise<void>;
}
