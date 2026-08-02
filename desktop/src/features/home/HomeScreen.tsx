import { useEffect, useState } from "react";
import { formatBytes, formatRelativeTime } from "./format";
import type {
  HomeAdapter,
  HomeNavigation,
  HomeSnapshot,
  ReadinessItem,
} from "./types";
import "./home.css";

export interface HomeScreenProps {
  adapter: HomeAdapter;
  navigation: HomeNavigation;
}

const quickActions = [
  {
    destination: "chat",
    glyph: "◌",
    title: "Start a chat",
    description: "Talk privately with a local model.",
  },
  {
    destination: "images",
    glyph: "◇",
    title: "Create an image",
    description: "Turn an idea into a visual.",
  },
  {
    destination: "voice",
    glyph: "≋",
    title: "Generate speech",
    description: "Give your words a local voice.",
  },
  {
    destination: "models",
    glyph: "⬡",
    title: "Find a model",
    description: "Browse models that fit this machine.",
  },
] as const;

export function HomeScreen({ adapter, navigation }: HomeScreenProps) {
  const [snapshot, setSnapshot] = useState<HomeSnapshot>();
  const [error, setError] = useState<string>();
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setError(undefined);
    adapter
      .getSnapshot(controller.signal)
      .then(setSnapshot)
      .catch((reason: unknown) => {
        if (controller.signal.aborted) return;
        setError(
          reason instanceof Error
            ? reason.message
            : "Home information is unavailable.",
        );
      });
    return () => controller.abort();
  }, [adapter, attempt]);

  if (error) {
    return (
      <section className="home-state" role="alert" aria-labelledby="home-error">
        <span className="home-state__mark" aria-hidden="true">
          !
        </span>
        <h1 id="home-error">Tapioca could not load your workspace.</h1>
        <p>{error}</p>
        <button type="button" onClick={() => setAttempt((value) => value + 1)}>
          Try again
        </button>
      </section>
    );
  }

  if (!snapshot) {
    return (
      <section className="home-loading" aria-busy="true" aria-label="Loading home">
        <div className="home-loading__hero" />
        <div className="home-loading__row">
          <div />
          <div />
          <div />
        </div>
      </section>
    );
  }

  return <HomeContent snapshot={snapshot} navigation={navigation} />;
}

export function HomeContent({
  snapshot,
  navigation,
}: {
  snapshot: HomeSnapshot;
  navigation: HomeNavigation;
}) {
  const completed = snapshot.readiness.filter(
    ({ state }) => state === "complete",
  ).length;
  const progress = snapshot.readiness.length
    ? completed / snapshot.readiness.length
    : 1;

  return (
    <section className="home-feature" aria-labelledby="home-title">
      <header className="home-hero">
        <div className="home-hero__copy">
          <p className="home-kicker">Your local AI workspace</p>
          <h1 id="home-title">
            Ready when <span>you are.</span>
          </h1>
          <p>
            Everything runs from this machine. Finish setup, choose a model,
            and start making.
          </p>
        </div>
        <div className="home-hero__readiness">
          <div className="home-progress">
            <span>
              <strong>{completed}</strong> of {snapshot.readiness.length} ready
            </span>
            <span>{Math.round(progress * 100)}%</span>
          </div>
          <div
            className="home-progress__track"
            role="progressbar"
            aria-label="Setup progress"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={Math.round(progress * 100)}
          >
            <span style={{ width: `${progress * 100}%` }} />
          </div>
          <div className="home-readiness">
            {snapshot.readiness.map((item) => (
              <ReadinessRow
                item={item}
                key={item.id}
                navigation={navigation}
              />
            ))}
          </div>
        </div>
      </header>

      <div className="home-summary-grid">
        <article className="home-summary-card">
          <div className="home-summary-card__head">
            <span aria-hidden="true">⌁</span>
            <p>Machine</p>
          </div>
          <h2>{snapshot.hardware.processor}</h2>
          <p>
            {snapshot.hardware.platform} ·{" "}
            {formatBytes(snapshot.hardware.memoryBytes)} memory
          </p>
          {snapshot.hardware.accelerator ? (
            <span className="home-badge">{snapshot.hardware.accelerator}</span>
          ) : null}
        </article>
        <article className="home-summary-card">
          <div className="home-summary-card__head">
            <span aria-hidden="true">◫</span>
            <p>Model storage</p>
          </div>
          <h2>{formatBytes(snapshot.storage.modelsBytes)} used</h2>
          <p>{formatBytes(snapshot.storage.availableBytes)} available</p>
          <span className="home-path" title={snapshot.storage.location}>
            {snapshot.storage.location}
          </span>
        </article>
      </div>

      <section className="home-section" aria-labelledby="quick-actions-title">
        <div className="home-section__heading">
          <div>
            <p className="home-kicker">Make something</p>
            <h2 id="quick-actions-title">Where do you want to begin?</h2>
          </div>
        </div>
        <div className="home-actions">
          {quickActions.map((action) => (
            <button
              className="home-action"
              key={action.destination}
              type="button"
              onClick={() => navigation.open(action.destination)}
            >
              <span className="home-action__glyph" aria-hidden="true">
                {action.glyph}
              </span>
              <span>
                <strong>{action.title}</strong>
                <small>{action.description}</small>
              </span>
              <span className="home-action__arrow" aria-hidden="true">
                →
              </span>
            </button>
          ))}
        </div>
      </section>

      <section className="home-section" aria-labelledby="activity-title">
        <div className="home-section__heading">
          <div>
            <p className="home-kicker">Pick up where you left off</p>
            <h2 id="activity-title">Recent activity</h2>
          </div>
        </div>
        {snapshot.recentActivity.length ? (
          <ul className="home-activity">
            {snapshot.recentActivity.map((activity) => (
              <li key={activity.id}>
                <button
                  type="button"
                  disabled={!activity.destination}
                  onClick={() =>
                    activity.destination &&
                    navigation.open(activity.destination)
                  }
                >
                  <span
                    className={`home-activity__kind home-activity__kind--${activity.kind}`}
                  >
                    {activity.kind.slice(0, 1).toUpperCase()}
                  </span>
                  <span className="home-activity__copy">
                    <strong>{activity.title}</strong>
                    <small>{activity.detail}</small>
                  </span>
                  <time dateTime={activity.occurredAt}>
                    {formatRelativeTime(activity.occurredAt)}
                  </time>
                </button>
              </li>
            ))}
          </ul>
        ) : (
          <div className="home-empty">
            <span aria-hidden="true">✦</span>
            <h3>Your first creation will appear here.</h3>
            <p>Start a chat, image, video, voice, or agent run.</p>
          </div>
        )}
      </section>
    </section>
  );
}

function ReadinessRow({
  item,
  navigation,
}: {
  item: ReadinessItem;
  navigation: HomeNavigation;
}) {
  return (
    <div className={`home-ready-row home-ready-row--${item.state}`}>
      <span className="home-ready-row__state" aria-hidden="true">
        {item.state === "complete" ? "✓" : item.state === "blocked" ? "!" : "·"}
      </span>
      <span>
        <strong>{item.title}</strong>
        <small>{item.description}</small>
      </span>
      {item.actionLabel && item.destination ? (
        <button
          type="button"
          onClick={() => navigation.open(item.destination!)}
        >
          {item.actionLabel}
        </button>
      ) : null}
    </div>
  );
}
