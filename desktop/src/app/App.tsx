import { useEffect, useMemo, useState } from "react";
import {
  hrefForRoute,
  navigationItems,
  routeFromHash,
  type RouteId,
} from "./navigation";
import { PlaceholderRoute } from "../routes/PlaceholderRoute";
import { HomeScreen } from "../features/home";
import { ModelsScreen, type MachineProfile } from "../features/models";
import { ChatWorkspace } from "../features/chat";
import { AgentCockpit } from "../features/agents";
import { CreatorScreen } from "../features/create";
import { createRendererAdapters, type RendererAdapters } from "./adapters";
import type { TapiocaDesktopApi } from "../shared/ipc";

type RuntimeState = "ready" | "starting" | "degraded" | "offline";

function useActiveRoute(): RouteId {
  const [route, setRoute] = useState(() => routeFromHash(window.location.hash));

  useEffect(() => {
    const syncRoute = () => setRoute(routeFromHash(window.location.hash));
    window.addEventListener("hashchange", syncRoute);
    return () => window.removeEventListener("hashchange", syncRoute);
  }, []);

  return route;
}

function useRuntimeStatus() {
  const [status, setStatus] = useState<RuntimeState>("starting");
  const [facts, setFacts] = useState("Connecting");

  useEffect(() => {
    let active = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const poll = async () => {
      try {
        const health = await window.tapioca.health();
        if (!active) return;
        setStatus(health.status);
        setFacts(`${health.controlVersion ? `Control ${health.controlVersion} · ` : ""}Protocol v${health.protocolVersion} · ${health.platform}/${health.arch}`);
      } catch {
        if (!active) return;
        setStatus("offline");
        setFacts("Runtime offline");
      } finally {
        if (active) timer = setTimeout(poll, 3_000);
      }
    };
    void poll();
    return () => {
      active = false;
      if (timer) clearTimeout(timer);
    };
  }, []);

  const [jobs, setJobs] = useState(0);
  useEffect(
    () =>
      window.tapioca.onJobEvent((event) => {
        if (event.event === "job.started") setJobs((value) => value + 1);
        if (event.event === "job.completed" || event.event === "job.failed") {
          setJobs((value) => Math.max(0, value - 1));
        }
      }),
    [],
  );

  return { status, facts, jobs };
}

export default function App() {
  const activeRouteId = useActiveRoute();
  const runtime = useRuntimeStatus();
  const activeRoute = useMemo(
    () =>
      navigationItems.find((item) => item.id === activeRouteId) ??
      navigationItems[0],
    [activeRouteId],
  );
  const adapters = useMemo(
    () =>
      createRendererAdapters(window.tapioca, (destination) => {
        window.location.hash = `#/${destination}`;
      }),
    [],
  );

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <a className="brand" href={hrefForRoute("home")} aria-label="Tapioca home">
          <img src="./tapioca-icon.png" alt="" />
          <span>
            <strong>Tapioca</strong>
            <small>Local AI</small>
          </span>
        </a>

        <nav aria-label="Primary navigation">
          <ul className="nav-list">
            {navigationItems.map((item) => (
              <li key={item.id}>
                <a
                  className={
                    item.id === activeRouteId
                      ? "nav-item nav-item--active"
                      : "nav-item"
                  }
                  href={hrefForRoute(item.id)}
                  aria-current={item.id === activeRouteId ? "page" : undefined}
                >
                  <span className="nav-item__glyph" aria-hidden="true">
                    {item.glyph}
                  </span>
                  <span>{item.label}</span>
                </a>
              </li>
            ))}
          </ul>
        </nav>

        <div className="sidebar__footer">
          <div className="profile-mark" aria-hidden="true">
            C
          </div>
          <span>
            <strong>Local workspace</strong>
            <small>Private by default</small>
          </span>
        </div>
      </aside>

      <div className="workspace">
        <header className="titlebar">
          <div className="titlebar__trail">
            <span>Tapioca</span>
            <span aria-hidden="true">/</span>
            <strong>{activeRoute.label}</strong>
          </div>
          <div className="titlebar__actions">
            <button type="button" aria-label="Search" disabled>
              ⌕
            </button>
            <button type="button" aria-label="Notifications" disabled>
              ◦
            </button>
          </div>
        </header>

        <main>
          <ActiveFeature
            adapters={adapters}
            route={activeRouteId}
            placeholder={activeRoute}
          />
        </main>

        <footer className="runtime-strip">
          <div className="runtime-strip__group">
            <span
              className={`status-dot status-dot--${runtime.status}`}
              aria-hidden="true"
            />
            <strong>Tapioca runtime</strong>
            <span>{runtime.facts}</span>
          </div>
          <div className="runtime-strip__metrics" aria-label="Runtime resources">
            <span>Local control</span>
            <span>Jobs {runtime.jobs}</span>
            <span className="runtime-strip__ready">
              {runtime.status === "ready" ? "Ready" : "Foundation mode"}
            </span>
          </div>
        </footer>
      </div>
    </div>
  );
}

function ActiveFeature({
  adapters,
  route,
  placeholder,
}: {
  adapters: RendererAdapters;
  route: RouteId;
  placeholder: (typeof navigationItems)[number];
}) {
  if (route === "home") {
    return (
      <HomeScreen
        adapter={adapters.home}
        navigation={adapters.homeNavigation}
      />
    );
  }
  if (route === "chat") return <ChatWorkspace adapter={adapters.chat} />;
  if (route === "agents") return <AgentCockpit adapter={adapters.agents} />;
  if (route === "models") return <ModelHubRoute adapters={adapters} />;
  if (route === "images") return <CreatorScreen adapter={adapters.creator} initialMode="image" modes={["image"]} />;
  if (route === "video") return <CreatorScreen adapter={adapters.creator} initialMode="video" modes={["video"]} />;
  if (route === "voice") return <CreatorScreen adapter={adapters.creator} initialMode="speech" modes={["speech", "voice-clone"]} />;
  if (route === "api") return <ApiRoute />;
  if (route === "settings") return <SettingsRoute />;
  return <PlaceholderRoute route={placeholder} />;
}

function ApiRoute() {
  return (
    <section className="route-info" aria-labelledby="api-title">
      <h1 id="api-title">Local API</h1>
      <p>Chat-compatible clients connect to the loopback endpoint below after a model server is started from Chat.</p>
      <dl>
        <div><dt>Base URL</dt><dd><code>http://127.0.0.1:11435/v1</code></dd></div>
        <div><dt>Network scope</dt><dd>Loopback only</dd></div>
        <div><dt>Protocol</dt><dd>OpenAI-compatible chat completions</dd></div>
      </dl>
      <p>Tapioca does not expose the control sidecar or filesystem through this page.</p>
    </section>
  );
}

function SettingsRoute() {
  const [snapshot, setSnapshot] = useState<Awaited<ReturnType<TapiocaDesktopApi["systemSnapshot"]>>>();
  const [error, setError] = useState<string>();
  useEffect(() => {
    let active = true;
    window.tapioca.systemSnapshot()
      .then((value) => active && setSnapshot(value))
      .catch((cause: unknown) => active && setError(cause instanceof Error ? cause.message : String(cause)));
    return () => { active = false; };
  }, []);
  return (
    <section className="route-info" aria-labelledby="settings-title">
      <h1 id="settings-title">Runtime settings</h1>
      {error && <p role="alert">{error}</p>}
      {!snapshot ? <p>Loading runtime facts…</p> : (
        <dl>
          <div><dt>Platform</dt><dd>{snapshot.platform} / {snapshot.arch}</dd></div>
          <div><dt>Model storage</dt><dd>{snapshot.modelsPath}</dd></div>
          <div><dt>CPU</dt><dd>{snapshot.cpuCount} logical cores</dd></div>
          <div><dt>Accelerators</dt><dd>{snapshot.accelerators.join(", ")}</dd></div>
          <div><dt>Memory</dt><dd>{Math.round(snapshot.memoryBytes / 1024 ** 3)} GiB</dd></div>
        </dl>
      )}
      <p>Network binding and model storage are controlled by the local runtime. This desktop build does not offer unsafe arbitrary path overrides.</p>
    </section>
  );
}

function ModelHubRoute({ adapters }: { adapters: RendererAdapters }) {
  const [machine, setMachine] = useState<MachineProfile>();
  const [error, setError] = useState<string>();
  useEffect(() => {
    let active = true;
    adapters
      .machine()
      .then((profile) => active && setMachine(profile))
      .catch((cause: unknown) => {
        if (active) setError(cause instanceof Error ? cause.message : String(cause));
      });
    return () => {
      active = false;
    };
  }, [adapters]);
  if (error) return <div className="route-error" role="alert">{error}</div>;
  if (!machine) return <div className="route-loading" aria-busy="true">Inspecting this machine…</div>;
  return <ModelsScreen adapter={adapters.models} machine={machine} />;
}
