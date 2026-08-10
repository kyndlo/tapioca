import type { ReactNode } from "react";
import type { RouteId } from "./navigation";

const iconShapes: Record<RouteId, ReactNode> = {
  home: (
    <>
      <path d="m3.5 10.5 8.5-7 8.5 7" />
      <path d="M5.5 9.5v10h13v-10M9.5 19.5v-6h5v6" />
    </>
  ),
  chat: (
    <>
      <path d="M5 5.5h14a2 2 0 0 1 2 2v7a2 2 0 0 1-2 2h-5l-4.5 3v-3H5a2 2 0 0 1-2-2v-7a2 2 0 0 1 2-2Z" />
      <path d="M7.5 10.8h.01M12 10.8h.01M16.5 10.8h.01" />
    </>
  ),
  images: (
    <>
      <rect x="3" y="4" width="18" height="16" rx="2.5" />
      <circle cx="8.2" cy="9" r="1.5" />
      <path d="m5 17 4.2-4 3.1 2.8 2.4-2.2L19 17.5" />
    </>
  ),
  video: (
    <>
      <rect x="3" y="5" width="18" height="14" rx="2.5" />
      <path d="m10 9 5 3-5 3Z" />
    </>
  ),
  voice: (
    <>
      <path d="M4 10v4M8 7v10M12 4v16M16 7v10M20 10v4" />
    </>
  ),
  agents: (
    <>
      <path d="m12 3 1.35 4.15L17.5 8.5l-4.15 1.35L12 14l-1.35-4.15L6.5 8.5l4.15-1.35L12 3Z" />
      <path d="m18.5 14 .75 2.25L21.5 17l-2.25.75L18.5 20l-.75-2.25L15.5 17l2.25-.75.75-2.25Z" />
    </>
  ),
  models: (
    <>
      <path d="m12 3 8 4.5v9L12 21l-8-4.5v-9L12 3Z" />
      <path d="m4.3 7.7 7.7 4.4 7.7-4.4M12 12.1V21" />
    </>
  ),
  api: (
    <>
      <path d="m8.5 6-6 6 6 6M15.5 6l6 6-6 6M13.5 4l-3 16" />
    </>
  ),
  settings: (
    <>
      <circle cx="12" cy="12" r="3" />
      <path d="M12 2.8v2M12 19.2v2M21.2 12h-2M4.8 12h-2M18.5 5.5l-1.4 1.4M6.9 17.1l-1.4 1.4M18.5 18.5l-1.4-1.4M6.9 6.9 5.5 5.5" />
      <circle cx="12" cy="12" r="7.2" />
    </>
  ),
};

export function NavigationIcon({ route }: { route: RouteId }) {
  return (
    <svg
      aria-hidden="true"
      className="nav-item__icon-svg"
      fill="none"
      viewBox="0 0 24 24"
    >
      {iconShapes[route]}
    </svg>
  );
}
