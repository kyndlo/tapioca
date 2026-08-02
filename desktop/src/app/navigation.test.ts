import { describe, expect, it } from "vitest";
import {
  hrefForRoute,
  navigationItems,
  routeFromHash,
  routeIds,
} from "./navigation";

describe("desktop navigation", () => {
  it("contains each approved destination exactly once", () => {
    expect(navigationItems.map(({ label }) => label)).toEqual([
      "Home",
      "Chat",
      "Images",
      "Video",
      "Voice",
      "Agents",
      "Models",
      "API",
      "Settings",
    ]);
    expect(new Set(navigationItems.map(({ id }) => id)).size).toBe(
      routeIds.length,
    );
  });

  it("round-trips known routes and falls back safely", () => {
    expect(routeFromHash(hrefForRoute("voice"))).toBe("voice");
    expect(routeFromHash("#/not-a-route")).toBe("home");
    expect(routeFromHash("")).toBe("home");
  });
});
