import type { NavigationItem } from "../app/navigation";

interface PlaceholderRouteProps {
  route: NavigationItem;
}

const homeCards = [
  {
    eyebrow: "Start a conversation",
    title: "Chat with a local model",
    copy: "Private, fast, and available without sending prompts to a cloud.",
  },
  {
    eyebrow: "Make something",
    title: "Create image, video, or voice",
    copy: "One workspace for the multimodal models running on your machine.",
  },
  {
    eyebrow: "Build with Tapioca",
    title: "Launch an agent or API",
    copy: "Use the same local runtime from your terminal, editor, or application.",
  },
] as const;

export function PlaceholderRoute({ route }: PlaceholderRouteProps) {
  const isHome = route.id === "home";

  return (
    <section className="route" aria-labelledby="route-title">
      <div className="route__header">
        <div>
          <p className="eyebrow">{isHome ? "Local AI, beautifully simple" : "Workspace"}</p>
          <h1 id="route-title">
            {isHome ? (
              <>
                Welcome back to <span>Tapioca.</span>
              </>
            ) : (
              route.label
            )}
          </h1>
          <p className="route__description">{route.description}</p>
        </div>
        <button className="quiet-button" type="button" disabled>
          Coming next
        </button>
      </div>

      {isHome ? (
        <>
          <div className="hero-card">
            <div className="hero-card__copy">
              <p className="eyebrow eyebrow--light">Runtime ready</p>
              <h2>Your models. Your machine. Your ideas.</h2>
              <p>
                Tapioca brings chat, media, voices, and agents together in one
                focused local workspace.
              </p>
              <button className="primary-button" type="button" disabled>
                Choose a model
              </button>
            </div>
            <div className="hero-card__orb" aria-hidden="true">
              <img src="./tapioca-logo.png" alt="" />
              <span className="orbit orbit--one" />
              <span className="orbit orbit--two" />
            </div>
          </div>

          <div className="card-grid">
            {homeCards.map((card) => (
              <article className="feature-card" key={card.title}>
                <p className="eyebrow">{card.eyebrow}</p>
                <h2>{card.title}</h2>
                <p>{card.copy}</p>
                <span className="feature-card__arrow" aria-hidden="true">
                  →
                </span>
              </article>
            ))}
          </div>
        </>
      ) : (
        <div className="empty-state">
          <div className="empty-state__mark" aria-hidden="true">
            {route.glyph}
          </div>
          <p className="eyebrow">{route.label} foundation</p>
          <h2>This space is ready for its feature workflow.</h2>
          <p>
            Navigation, security boundaries, runtime contracts, and shared
            design tokens are in place. Product functionality arrives next.
          </p>
        </div>
      )}
    </section>
  );
}
