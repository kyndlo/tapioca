import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  applyAgentEvent,
  canLaunchAgent,
  initialAgentRunView,
  maskEnvironment,
} from "./state";
import type {
  AgentAdapter,
  AgentDefinition,
  AgentEnvironmentEntry,
  AgentLaunchRequest,
  AgentModel,
  AgentReadiness,
  WorkspaceSelection,
} from "./types";
import "./agents.css";

export interface AgentCockpitProps {
  adapter: AgentAdapter;
}

export function AgentCockpit({ adapter }: AgentCockpitProps) {
  const [definitions, setDefinitions] = useState<AgentDefinition[]>([]);
  const [models, setModels] = useState<AgentModel[]>([]);
  const [readiness, setReadiness] = useState<AgentReadiness>({
    server: "starting",
  });
  const [agentId, setAgentId] = useState<AgentDefinition["id"]>("codex");
  const [modelId, setModelId] = useState("");
  const [workspace, setWorkspace] = useState<WorkspaceSelection>();
  const [environment, setEnvironment] = useState<AgentEnvironmentEntry[]>([]);
  const [run, setRun] = useState(initialAgentRunView);
  const [runId, setRunId] = useState<string>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const logEnd = useRef<HTMLDivElement>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const [nextDefinitions, nextModels, nextReadiness] = await Promise.all([
        adapter.definitions(),
        adapter.models(),
        adapter.readiness(),
      ]);
      setDefinitions(nextDefinitions);
      setModels(nextModels);
      setReadiness(nextReadiness);
      setAgentId((current) =>
        nextDefinitions.some((agent) => agent.id === current)
          ? current
          : (nextDefinitions[0]?.id ?? "codex"),
      );
      setModelId((current) =>
        nextModels.some((model) => model.id === current)
          ? current
          : (nextModels.find((model) => model.ready)?.id ?? nextModels[0]?.id ?? ""),
      );
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
      setReadiness({ server: "offline", message: "Unable to inspect runtime." });
    } finally {
      setLoading(false);
    }
  }, [adapter]);

  useEffect(() => void refresh(), [refresh]);
  useEffect(() => {
    logEnd.current?.scrollIntoView({ block: "end" });
  }, [run.logs, run.status]);

  const selectedAgent = useMemo(
    () => definitions.find((agent) => agent.id === agentId),
    [agentId, definitions],
  );
  const selectedModel = useMemo(
    () => models.find((model) => model.id === modelId),
    [modelId, models],
  );
  const busy = ["checking", "launching", "running", "stopping"].includes(
    run.status,
  );
  const launchReady = canLaunchAgent({
    installed: selectedAgent?.installed ?? false,
    modelReady: selectedModel?.ready ?? false,
    serverReady: readiness.server === "ready",
    hasWorkspace: Boolean(workspace),
    busy,
  });

  const requestForSelection = (): AgentLaunchRequest | undefined => {
    if (!workspace || !modelId) return undefined;
    return { agent: agentId, modelId, workspace };
  };

  const previewEnvironment = async () => {
    const request = requestForSelection();
    if (!request) {
      setEnvironment([]);
      return;
    }
    setError(undefined);
    try {
      setEnvironment(maskEnvironment(await adapter.environment(request)));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  useEffect(() => {
    void previewEnvironment();
    // The preview is recalculated only from the selected launch identity.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [agentId, modelId, workspace?.path]);

  const chooseWorkspace = async () => {
    setError(undefined);
    try {
      const selection = await adapter.chooseWorkspace();
      if (selection) setWorkspace(selection);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  const launch = async () => {
    const request = requestForSelection();
    if (!request || !launchReady) return;
    setError(undefined);
    setRun({ status: "launching", logs: [] });
    try {
      const result = await adapter.launch(request, (event) => {
        setRun((current) => applyAgentEvent(current, event));
      });
      setRunId(result.runId);
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : String(cause);
      setRun((current) => ({ ...current, status: "failed", message }));
      setError(message);
    }
  };

  const stop = async () => {
    if (!runId) return;
    setRun((current) => ({
      ...current,
      status: "stopping",
      message: "Stopping agent…",
    }));
    try {
      await adapter.stop(runId);
      setRun((current) => ({
        ...current,
        status: "completed",
        message: "Agent stopped.",
      }));
      setRunId(undefined);
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : String(cause);
      setRun((current) => ({ ...current, status: "failed", message }));
      setError(message);
    }
  };

  return (
    <section className="agent-cockpit" aria-label="Agent cockpit">
      <header className="agent-header">
        <div>
          <span className="agent-kicker">Tool-using local models</span>
          <h1>Agent Cockpit</h1>
          <p>Launch supported coding agents with a reviewed local configuration.</p>
        </div>
        <div className={`agent-readiness agent-readiness--${readiness.server}`}>
          <span aria-hidden="true" />
          <div>
            <strong>Local API · {readiness.server}</strong>
            <small>{readiness.endpoint ?? readiness.message ?? "Checking endpoint…"}</small>
          </div>
          {readiness.server !== "ready" && (
            <button type="button" onClick={() => void refresh()} disabled={loading}>
              Check again
            </button>
          )}
        </div>
      </header>

      {error && (
        <div className="agent-error" role="alert">
          <span>{error}</span>
          <button type="button" onClick={() => setError(undefined)}>
            Dismiss
          </button>
        </div>
      )}

      <div className="agent-layout" aria-busy={loading}>
        <div className="agent-setup">
          <section className="agent-card">
            <div className="agent-card__heading">
              <div>
                <span className="agent-step">01</span>
                <h2>Choose an agent</h2>
              </div>
              <small>No custom commands</small>
            </div>
            {loading ? (
              <div className="agent-skeleton">Loading installed agents…</div>
            ) : definitions.length ? (
              <div className="agent-grid" role="radiogroup" aria-label="Agent">
                {definitions.map((agent) => (
                  <button
                    type="button"
                    role="radio"
                    aria-checked={agent.id === agentId}
                    className={agent.id === agentId ? "is-selected" : ""}
                    key={agent.id}
                    onClick={() => setAgentId(agent.id)}
                    disabled={busy}
                  >
                    <AgentGlyph id={agent.id} />
                    <span className="agent-choice__details">
                      <strong>{agent.name}</strong>
                      <small className="agent-choice__description" title={agent.description}>
                        {agent.description}
                      </small>
                    </span>
                    <em className={agent.installed ? "is-ready" : ""}>
                      {agent.installed ? "Installed" : "Not installed"}
                    </em>
                  </button>
                ))}
              </div>
            ) : (
              <div className="agent-empty">
                No supported coding agents were found on this machine.
              </div>
            )}
          </section>

          <section className="agent-card agent-card--configuration">
            <div className="agent-card__heading">
              <div>
                <span className="agent-step">02</span>
                <h2>Configure launch</h2>
              </div>
            </div>
            <div className="agent-fields">
              <label>
                <span>Local model</span>
                <select
                  value={modelId}
                  onChange={(event) => setModelId(event.target.value)}
                  disabled={busy || models.length === 0}
                >
                  {models.length === 0 && <option value="">No models available</option>}
                  {models.map((model) => (
                    <option key={model.id} value={model.id} disabled={!model.ready}>
                      {model.name}{model.ready ? "" : " — unavailable"}
                    </option>
                  ))}
                </select>
                <small>{selectedModel?.context ?? "Select a tool-capable model."}</small>
              </label>
              <div className="agent-workspace-field">
                <span>Workspace</span>
                <button type="button" onClick={() => void chooseWorkspace()} disabled={busy}>
                  {workspace ? "Change folder" : "Choose folder"}
                </button>
                <strong>{workspace?.displayName ?? "No workspace selected"}</strong>
                <small>{workspace?.path ?? "The agent is scoped to the chosen folder."}</small>
              </div>
            </div>
          </section>

          <section className="agent-card">
            <div className="agent-card__heading">
              <div>
                <span className="agent-step">03</span>
                <h2>Review environment</h2>
              </div>
              <small>Read-only preview</small>
            </div>
            {environment.length ? (
              <dl className="agent-environment">
                {environment.map((entry) => (
                  <div key={entry.name}>
                    <dt>{entry.name}</dt>
                    <dd>{entry.value}</dd>
                  </div>
                ))}
              </dl>
            ) : (
              <p className="agent-empty">
                Choose a model and workspace to preview the fixed launch environment.
              </p>
            )}
          </section>

          <div className="agent-launchbar">
            <div>
              <strong>{selectedAgent?.name ?? "Choose an agent"}</strong>
              <span>
                {!selectedAgent?.installed
                  ? "Install the selected agent first."
                  : readiness.server !== "ready"
                    ? "Waiting for the local API."
                    : !workspace
                      ? "Choose a workspace to continue."
                      : selectedModel?.ready
                        ? "Configuration ready."
                        : "Choose a ready model."}
              </span>
            </div>
            {busy ? (
              <button
                type="button"
                className="agent-stop"
                onClick={() => void stop()}
                disabled={!runId || run.status === "stopping"}
              >
                {run.status === "stopping" ? "Stopping…" : "Stop agent"}
              </button>
            ) : (
              <button
                type="button"
                className="agent-launch"
                onClick={() => void launch()}
                disabled={!launchReady}
              >
                Launch agent
              </button>
            )}
          </div>
        </div>

        <aside className="agent-console" aria-label="Agent output">
          <header>
            <div>
              <span className={`agent-status agent-status--${run.status}`} aria-hidden="true" />
              <strong>{run.status === "idle" ? "Launch output" : run.status}</strong>
            </div>
            <button
              type="button"
              onClick={() => setRun(initialAgentRunView())}
              disabled={busy || run.logs.length === 0}
            >
              Clear
            </button>
          </header>
          <div className="agent-console__body" role="log" aria-live="polite">
            {run.logs.length === 0 ? (
              <div className="agent-console__empty">
                <span aria-hidden="true">&gt;_</span>
                <p>Agent status and sanitized output will appear here.</p>
              </div>
            ) : (
              run.logs.map((line) => (
                <p key={line.id} data-level={line.level}>
                  <span>{line.level}</span>
                  {line.message}
                </p>
              ))
            )}
            {run.message && <div className="agent-console__status">{run.message}</div>}
            <div ref={logEnd} />
          </div>
        </aside>
      </div>
    </section>
  );
}

function AgentGlyph({ id }: { id: AgentDefinition["id"] }) {
  const glyphs: Record<AgentDefinition["id"], string> = {
    codex: "CX",
    claude: "CL",
    opencode: "OC",
    openclaw: "OW",
    hermes: "HE",
  };
  return <span className="agent-glyph" aria-hidden="true">{glyphs[id]}</span>;
}
