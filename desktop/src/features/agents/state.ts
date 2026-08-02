import type {
  AgentEnvironmentEntry,
  AgentRunEvent,
  AgentRunStatus,
} from "./types";

export interface AgentLogLine {
  id: number;
  level: "info" | "warn" | "error";
  message: string;
}

export interface AgentRunView {
  status: AgentRunStatus;
  message?: string;
  logs: AgentLogLine[];
}

export const initialAgentRunView = (): AgentRunView => ({
  status: "idle",
  logs: [],
});

export function applyAgentEvent(
  state: AgentRunView,
  event: AgentRunEvent,
): AgentRunView {
  if (event.type === "status") {
    return { ...state, status: event.status, message: event.message };
  }
  if (event.type === "exit") {
    return {
      ...state,
      status: event.code === 0 ? "completed" : "failed",
      message:
        event.code === 0 ? "Agent session completed." : `Agent exited (${event.code ?? "signal"}).`,
    };
  }
  return {
    ...state,
    logs: [
      ...state.logs.slice(-499),
      { id: state.logs.length + 1, level: event.level, message: event.message },
    ],
  };
}

export function maskEnvironment(
  entries: AgentEnvironmentEntry[],
): AgentEnvironmentEntry[] {
  return entries.map((entry) => ({
    ...entry,
    value: entry.sensitive ? "••••••••" : entry.value,
  }));
}

export function canLaunchAgent(input: {
  installed: boolean;
  modelReady: boolean;
  serverReady: boolean;
  hasWorkspace: boolean;
  busy: boolean;
}): boolean {
  return (
    input.installed &&
    input.modelReady &&
    input.serverReady &&
    input.hasWorkspace &&
    !input.busy
  );
}
