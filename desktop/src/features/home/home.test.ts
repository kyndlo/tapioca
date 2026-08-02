import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { HomeScreen } from "./HomeScreen";
import { formatBytes, formatRelativeTime } from "./format";
import type { HomeSnapshot } from "./types";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT =
  true;

const snapshot: HomeSnapshot = {
  readiness: [
    {
      id: "runtime",
      title: "Runtime connected",
      description: "Ready",
      state: "complete",
    },
    {
      id: "model",
      title: "Choose a model",
      description: "One more step",
      state: "current",
      actionLabel: "Browse",
      destination: "models",
    },
  ],
  hardware: {
    platform: "macOS",
    processor: "Apple Silicon",
    memoryBytes: 32 * 1024 ** 3,
    accelerator: "Metal",
  },
  storage: {
    modelsBytes: 8 * 1024 ** 3,
    availableBytes: 120 * 1024 ** 3,
    location: "/models",
  },
  recentActivity: [],
};

let container: HTMLDivElement | undefined;
let root: ReturnType<typeof createRoot> | undefined;

afterEach(async () => {
  if (root) await act(() => root?.unmount());
  container?.remove();
  root = undefined;
  container = undefined;
});

describe("home formatting", () => {
  it("formats storage and relative time for beginner-readable summaries", () => {
    expect(formatBytes(32 * 1024 ** 3)).toBe("32 GB");
    expect(
      formatRelativeTime(
        "2026-08-01T11:30:00.000Z",
        new Date("2026-08-01T12:00:00.000Z"),
      ),
    ).toBe("30m ago");
  });
});

describe("HomeScreen", () => {
  it("loads readiness and routes its next-step action", async () => {
    const open = vi.fn();
    container = document.createElement("div");
    document.body.append(container);
    root = createRoot(container);

    await act(async () => {
      root?.render(
        createElement(HomeScreen, {
          adapter: { getSnapshot: vi.fn().mockResolvedValue(snapshot) },
          navigation: { open },
        }),
      );
    });

    expect(container.textContent).toContain("Ready when you are.");
    expect(container.textContent).toContain("1 of 2 ready");
    const browse = Array.from(container.querySelectorAll("button")).find(
      (button) => button.textContent === "Browse",
    );
    await act(() => browse?.click());
    expect(open).toHaveBeenCalledWith("models");
  });

  it("exposes a retry action when the adapter fails", async () => {
    container = document.createElement("div");
    document.body.append(container);
    root = createRoot(container);
    const getSnapshot = vi
      .fn()
      .mockRejectedValueOnce(new Error("Runtime unavailable"))
      .mockResolvedValueOnce(snapshot);

    await act(async () => {
      root?.render(
        createElement(HomeScreen, {
          adapter: { getSnapshot },
          navigation: { open: vi.fn() },
        }),
      );
    });
    expect(container.getAttribute("role")).toBeNull();
    expect(container.textContent).toContain("Runtime unavailable");
    const retry = Array.from(container.querySelectorAll("button")).find(
      (button) => button.textContent === "Try again",
    );
    await act(async () => retry?.click());
    expect(container.textContent).toContain("Ready when you are.");
  });
});
