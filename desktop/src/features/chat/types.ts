export type ChatConnectionState =
  | "ready"
  | "startable"
  | "connecting"
  | "offline";
export type ChatMessageRole = "user" | "assistant";

export interface ChatModel {
  id: string;
  name: string;
  detail?: string;
  ready: boolean;
}

export interface ChatSessionSummary {
  id: string;
  title: string;
  updatedAt: string;
}

export interface ChatToolStep {
  id: string;
  name: string;
  status: "requested" | "running" | "completed" | "failed";
  summary?: string;
}

export interface ChatMessage {
  id: string;
  role: ChatMessageRole;
  content: string;
  thinking?: string;
  tools?: ChatToolStep[];
  createdAt: string;
}

export interface ChatSession {
  id: string;
  title: string;
  modelId?: string;
  messages: ChatMessage[];
}

export type ChatStreamEvent =
  | { type: "thinking.delta"; text: string }
  | { type: "content.delta"; text: string }
  | { type: "tool"; tool: ChatToolStep }
  | { type: "completed" }
  | { type: "error"; message: string; retryable: boolean };

export interface ChatSendRequest {
  sessionId?: string;
  modelId: string;
  messages: Array<Pick<ChatMessage, "role" | "content">>;
}

export interface ChatAdapter {
  connection(): Promise<ChatConnectionState>;
  reconnect(): Promise<ChatConnectionState>;
  models(): Promise<ChatModel[]>;
  sessions(): Promise<ChatSessionSummary[]>;
  loadSession(id: string): Promise<ChatSession>;
  send(
    request: ChatSendRequest,
    onEvent: (event: ChatStreamEvent) => void,
  ): Promise<{ sessionId: string; assistantMessageId: string }>;
  stop(): Promise<void>;
}
