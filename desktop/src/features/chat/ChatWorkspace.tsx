import {
  type FormEvent,
  type KeyboardEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  applyStreamEvent,
  chatMessagesForRequest,
  emptyChatDraft,
  isByeCommand,
  lastUserPrompt,
  type ChatDraft,
} from "./state";
import type {
  ChatAdapter,
  ChatConnectionState,
  ChatMessage,
  ChatModel,
  ChatSessionSummary,
} from "./types";
import "./chat.css";

export interface ChatWorkspaceProps {
  adapter: ChatAdapter;
}

const now = () => new Date().toISOString();

export function ChatWorkspace({ adapter }: ChatWorkspaceProps) {
  const [connection, setConnection] =
    useState<ChatConnectionState>("connecting");
  const [models, setModels] = useState<ChatModel[]>([]);
  const [modelId, setModelId] = useState("");
  const [sessions, setSessions] = useState<ChatSessionSummary[]>([]);
  const [sessionId, setSessionId] = useState<string>();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState<ChatDraft>();
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [closed, setClosed] = useState(false);
  const [error, setError] = useState<string>();
  const streamRef = useRef<HTMLDivElement>(null);

  const refresh = useCallback(async () => {
    setError(undefined);
    setConnection("connecting");
    try {
      const [nextConnection, nextModels, nextSessions] = await Promise.all([
        adapter.connection(),
        adapter.models(),
        adapter.sessions(),
      ]);
      setConnection(nextConnection);
      setModels(nextModels);
      setSessions(nextSessions);
      setModelId((current) => {
        if (nextModels.some((model) => model.id === current)) return current;
        return nextModels.find((model) => model.ready)?.id ?? nextModels[0]?.id ?? "";
      });
    } catch (cause) {
      setConnection("offline");
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  }, [adapter]);

  useEffect(() => void refresh(), [refresh]);
  useEffect(() => {
    streamRef.current?.scrollIntoView({ block: "end", behavior: "smooth" });
  }, [messages, draft]);

  const selectedModel = useMemo(
    () => models.find((model) => model.id === modelId),
    [modelId, models],
  );

  const openSession = async (id: string) => {
    if (busy) return;
    setError(undefined);
    try {
      const session = await adapter.loadSession(id);
      setSessionId(session.id);
      setMessages(session.messages);
      if (session.modelId) setModelId(session.modelId);
      setClosed(false);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  const sendPrompt = async (prompt: string) => {
    const text = prompt.trim();
    if (!text || busy) return;
    if (isByeCommand(text)) {
      try {
        await adapter.stop();
        setBusy(false);
        setDraft(undefined);
        setClosed(true);
        setConnection("startable");
        setInput("");
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : String(cause));
      }
      return;
    }
    if (!selectedModel?.ready) {
      setError("Choose an installed model before sending.");
      return;
    }

    const userMessage: ChatMessage = {
      id: crypto.randomUUID(),
      role: "user",
      content: text,
      createdAt: now(),
    };
    const requestMessages = [...messages, userMessage];
    setMessages(requestMessages);
    setInput("");
    setError(undefined);
    setClosed(false);
    setBusy(true);
    setDraft(emptyChatDraft());

    let accumulated = emptyChatDraft();
    try {
      const result = await adapter.send(
        {
          sessionId,
          modelId: selectedModel.id,
          messages: chatMessagesForRequest(requestMessages),
        },
        (event) => {
          accumulated = applyStreamEvent(accumulated, event);
          setDraft(accumulated);
        },
      );
      setConnection("ready");
      setSessionId(result.sessionId);
      if (accumulated.error) throw new Error(accumulated.error);
      setMessages((current) => [
        ...current,
        {
          id: result.assistantMessageId,
          role: "assistant",
          content: accumulated.content,
          thinking: accumulated.thinking || undefined,
          tools: accumulated.tools.length ? accumulated.tools : undefined,
          createdAt: now(),
        },
      ]);
      setDraft(undefined);
      setSessions(await adapter.sessions());
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
      setDraft((current) => current && { ...current, complete: true });
    } finally {
      setBusy(false);
    }
  };

  const onSubmit = (event: FormEvent) => {
    event.preventDefault();
    void sendPrompt(input);
  };

  const onInputKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      void sendPrompt(input);
    }
  };

  const stop = async () => {
    await adapter.stop().catch((cause) => {
      setError(cause instanceof Error ? cause.message : String(cause));
    });
    setBusy(false);
    setConnection("startable");
    setDraft((current) => current && { ...current, complete: true });
  };

  const reconnect = async () => {
    setConnection("connecting");
    setError(undefined);
    try {
      setConnection(await adapter.reconnect());
      await refresh();
    } catch (cause) {
      setConnection("offline");
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  const regenerate = () => {
    const prompt = lastUserPrompt(messages);
    if (prompt) void sendPrompt(prompt);
  };

  return (
    <section className="chat-workspace" aria-label="Local model chat">
      <aside className="chat-history" aria-label="Chat history">
        <div className="chat-history__heading">
          <div>
            <span className="chat-kicker">Conversations</span>
            <strong>History</strong>
          </div>
          <button
            type="button"
            onClick={() => {
              setSessionId(undefined);
              setMessages([]);
              setDraft(undefined);
              setClosed(false);
              setError(undefined);
            }}
            aria-label="Start a new conversation"
          >
            +
          </button>
        </div>
        {sessions.length ? (
          <ul>
            {sessions.map((session) => (
              <li key={session.id}>
                <button
                  type="button"
                  className={session.id === sessionId ? "is-active" : ""}
                  onClick={() => void openSession(session.id)}
                  disabled={busy}
                >
                  <span>{session.title}</span>
                  <time dateTime={session.updatedAt}>
                    {new Date(session.updatedAt).toLocaleDateString()}
                  </time>
                </button>
              </li>
            ))}
          </ul>
        ) : (
          <p className="chat-muted">Your local conversations will appear here.</p>
        )}
      </aside>

      <div className="chat-panel">
        <header className="chat-panel__header">
          <div>
            <span className="chat-kicker">Private conversation</span>
            <h1>Chat</h1>
          </div>
          <label>
            <span>Model</span>
            <select
              value={modelId}
              onChange={(event) => setModelId(event.target.value)}
              disabled={busy || models.length === 0}
              aria-label="Chat model"
            >
              {models.length === 0 && <option value="">No models available</option>}
              {models.map((model) => (
                <option key={model.id} value={model.id} disabled={!model.ready}>
                  {model.name}{model.ready ? "" : " — unavailable"}
                </option>
              ))}
            </select>
          </label>
        </header>

        {connection !== "ready" && (
          <div className="chat-notice" role="status">
            <span>
              {connection === "connecting"
                ? "Connecting to the local runtime…"
                : connection === "startable"
                  ? "Local runtime is ready. The selected model server will start when you send."
                  : "The Tapioca control sidecar is offline."}
            </span>
            {connection === "offline" && (
              <button type="button" onClick={() => void reconnect()}>
                Reconnect
              </button>
            )}
          </div>
        )}

        <div className="chat-transcript" aria-live="polite" aria-busy={busy}>
          {messages.length === 0 && !draft && (
            <div className="chat-empty">
              <span aria-hidden="true">◌</span>
              <h2>What are you working on?</h2>
              <p>Choose a local model, then start a private conversation.</p>
            </div>
          )}
          {messages.map((message) => (
            <article
              className={`chat-message chat-message--${message.role}`}
              key={message.id}
            >
              <div className="chat-message__meta">
                <strong>{message.role === "user" ? "You" : "Tapioca"}</strong>
                <button
                  type="button"
                  onClick={() => void navigator.clipboard.writeText(message.content)}
                  aria-label={`Copy ${message.role} message`}
                >
                  Copy
                </button>
              </div>
              {message.thinking && (
                <details className="chat-disclosure">
                  <summary>Thinking</summary>
                  <pre>{message.thinking}</pre>
                </details>
              )}
              {message.tools && message.tools.length > 0 && (
                <details className="chat-disclosure">
                  <summary>Tool flow · {message.tools.length} steps</summary>
                  <ToolFlow tools={message.tools} />
                </details>
              )}
              <p>{message.content}</p>
            </article>
          ))}
          {draft && (
            <article className="chat-message chat-message--assistant">
              <div className="chat-message__meta">
                <strong>Tapioca</strong>
                <span className="chat-streaming">{busy ? "Generating" : "Paused"}</span>
              </div>
              {draft.thinking && (
                <details className="chat-disclosure" open>
                  <summary>Thinking</summary>
                  <pre>{draft.thinking}</pre>
                </details>
              )}
              {draft.tools.length > 0 && (
                <details className="chat-disclosure" open>
                  <summary>Tool flow · {draft.tools.length} steps</summary>
                  <ToolFlow tools={draft.tools} />
                </details>
              )}
              <p>{draft.content || "…"}</p>
            </article>
          )}
          <div ref={streamRef} />
        </div>

        {error && (
          <div className="chat-error" role="alert">
            <span>{error}</span>
            <button type="button" onClick={regenerate} disabled={busy}>
              Try again
            </button>
          </div>
        )}
        {closed ? (
          <div className="chat-closed" role="status">
            <span>Conversation ended with /bye.</span>
            <button type="button" onClick={() => setClosed(false)}>
              Start again
            </button>
          </div>
        ) : (
          <form className="chat-composer" onSubmit={onSubmit}>
            <label htmlFor="chat-prompt">Message your local model</label>
            <textarea
              id="chat-prompt"
              value={input}
              onChange={(event) => setInput(event.target.value)}
              onKeyDown={onInputKeyDown}
              placeholder="Ask anything…  (/bye ends the conversation)"
              rows={2}
              disabled={connection === "offline" || connection === "connecting"}
            />
            <div className="chat-composer__actions">
              <span>Enter to send · Shift+Enter for a new line</span>
              {busy ? (
                <button type="button" className="chat-stop" onClick={() => void stop()}>
                  Stop
                </button>
              ) : (
                <button
                  type="submit"
                  className="chat-send"
                  disabled={!input.trim() || !selectedModel?.ready}
                >
                  Send
                </button>
              )}
            </div>
          </form>
        )}
      </div>
    </section>
  );
}

function ToolFlow({ tools }: { tools: NonNullable<ChatMessage["tools"]> }) {
  return (
    <ol className="chat-tools">
      {tools.map((tool) => (
        <li key={tool.id} data-status={tool.status}>
          <span aria-hidden="true" />
          <div>
            <strong>{tool.name}</strong>
            {tool.summary && <p>{tool.summary}</p>}
          </div>
          <small>{tool.status}</small>
        </li>
      ))}
    </ol>
  );
}
