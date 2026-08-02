export const routeIds = [
  "home",
  "chat",
  "images",
  "video",
  "voice",
  "agents",
  "models",
  "api",
  "settings",
] as const;

export type RouteId = (typeof routeIds)[number];

export interface NavigationItem {
  id: RouteId;
  label: string;
  glyph: string;
  description: string;
}

export const navigationItems: readonly NavigationItem[] = [
  {
    id: "home",
    label: "Home",
    glyph: "⌂",
    description: "Your local AI workspace",
  },
  {
    id: "chat",
    label: "Chat",
    glyph: "◌",
    description: "Talk with local language models",
  },
  {
    id: "images",
    label: "Images",
    glyph: "◇",
    description: "Generate and edit images locally",
  },
  {
    id: "video",
    label: "Video",
    glyph: "▷",
    description: "Create motion from prompts and images",
  },
  {
    id: "voice",
    label: "Voice",
    glyph: "≋",
    description: "Speech synthesis and saved voices",
  },
  {
    id: "agents",
    label: "Agents",
    glyph: "✦",
    description: "Launch coding and tool-using agents",
  },
  {
    id: "models",
    label: "Models",
    glyph: "⬡",
    description: "Browse and manage local models",
  },
  {
    id: "api",
    label: "API",
    glyph: "‹›",
    description: "Connect apps to the local endpoint",
  },
  {
    id: "settings",
    label: "Settings",
    glyph: "⚙",
    description: "Runtime, storage, and preferences",
  },
] as const;

export function isRouteId(value: string): value is RouteId {
  return routeIds.includes(value as RouteId);
}

export function routeFromHash(hash: string): RouteId {
  const candidate = hash.replace(/^#\/?/, "").toLowerCase();
  return isRouteId(candidate) ? candidate : "home";
}

export function hrefForRoute(route: RouteId): string {
  return `#/${route}`;
}
