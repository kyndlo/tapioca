import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ModelsScreen } from "./ModelsScreen";
import {
  estimatedDiskAfterInstall,
  downloadPercent,
  modelCompatibility,
} from "./model-utils";
import type {
  MachineProfile,
  ModelHubAdapter,
  ModelRecord,
  PullOptions,
} from "./types";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT =
  true;

const gib = 1024 ** 3;
const machine: MachineProfile = {
  platform: "macos",
  memoryBytes: 32 * gib,
  availableDiskBytes: 100 * gib,
  accelerators: ["apple"],
};
const model: ModelRecord = {
  id: "pearl-chat",
  name: "Pearl Chat 8B",
  creator: "Tapioca Community",
  description: "A fast local conversation model.",
  kind: "chat",
  backend: "mlx",
  tags: ["tools", "conversation"],
  requirements: {
    memoryBytes: 12 * gib,
    recommendedMemoryBytes: 16 * gib,
    diskBytes: 6 * gib,
    platforms: ["macos"],
    accelerators: ["apple"],
  },
  installed: false,
};

let container: HTMLDivElement | undefined;
let root: ReturnType<typeof createRoot> | undefined;

afterEach(async () => {
  if (root) await act(() => root?.unmount());
  container?.remove();
  root = undefined;
  container = undefined;
});

describe("model compatibility", () => {
  it("does not reserve download space again for installed models", () => {
    const installed = { ...model, installed: true };
    const lowDisk = { ...machine, availableDiskBytes: 1 * gib };
    expect(modelCompatibility(installed, lowDisk).level).toBe("compatible");
    expect(estimatedDiskAfterInstall(installed, lowDisk).remaining).toBe("1.0 GB");
    expect(modelCompatibility(model, lowDisk).reasons).toContain("Not enough disk space");
  });

  it("handles unknown download sizes and clamps progress", () => {
    expect(downloadPercent(0, 0)).toBeUndefined();
    expect(downloadPercent(10, Number.NaN)).toBeUndefined();
    expect(downloadPercent(-10, 100)).toBe(0);
    expect(downloadPercent(200, 100)).toBe(100);
  });
  it("uses structured machine requirements and produces a disk estimate", () => {
    expect(modelCompatibility(model, machine)).toEqual({
      level: "compatible",
      reasons: ["Good fit for this machine"],
    });
    expect(estimatedDiskAfterInstall(model, machine)).toEqual({
      required: "6.0 GB",
      remaining: "94 GB",
    });
  });

  it("reports every incompatible constraint", () => {
    expect(
      modelCompatibility(model, {
        platform: "windows",
        memoryBytes: 8 * gib,
        availableDiskBytes: 2 * gib,
        accelerators: ["intel"],
      }),
    ).toMatchObject({
      level: "incompatible",
      reasons: [
        "Not available for windows",
        "Not enough memory",
        "Not enough disk space",
        "No supported accelerator",
      ],
    });
  });
});

describe("ModelsScreen", () => {
  it("traps dialog focus and Escape closes only the top dialog", async () => {
    const adapter: ModelHubAdapter = { listModels: vi.fn().mockResolvedValue([{ ...model, installed: true }]), pullModel: vi.fn(), cancelPull: vi.fn(), removeModel: vi.fn() };
    container = document.createElement("div"); document.body.append(container); root = createRoot(container);
    await act(async () => root?.render(createElement(ModelsScreen, { adapter, machine })));
    const manage = container.querySelector<HTMLButtonElement>(".model-secondary")!;
    manage.focus();
    await act(() => manage.click());
    const close = container.querySelector<HTMLButtonElement>(".model-modal-close")!;
    expect(document.activeElement).toBe(close);
    await act(() => window.dispatchEvent(new KeyboardEvent("keydown", { key: "Tab", shiftKey: true, cancelable: true })));
    expect(document.activeElement?.textContent).toBe("Remove model");
    await act(() => (document.activeElement as HTMLButtonElement).click());
    expect(container.querySelector("[role=alertdialog]")).not.toBeNull();
    await act(() => window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" })));
    expect(container.querySelector("[role=alertdialog]")).toBeNull();
    expect(container.querySelector("[role=dialog]")).not.toBeNull();
    await act(() => window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" })));
    expect(container.querySelector("[role=dialog]")).toBeNull();
    expect(document.activeElement).toBe(manage);
  });
  it("filters installed models and resets all filters", async () => {
    const adapter: ModelHubAdapter = {
      listModels: vi.fn().mockResolvedValue([model, { ...model, id: "installed", name: "Installed model", installed: true }]),
      pullModel: vi.fn(), cancelPull: vi.fn(), removeModel: vi.fn(),
    };
    container = document.createElement("div");
    document.body.append(container);
    root = createRoot(container);
    await act(async () => root?.render(createElement(ModelsScreen, { adapter, machine })));
    const checkbox = Array.from(container.querySelectorAll("label")).find((item) => item.textContent === "Installed only")?.querySelector("input");
    await act(() => checkbox?.click());
    expect(container.querySelectorAll(".model-card")).toHaveLength(1);
    expect(container.textContent).toContain("1 of 2 models");
    const reset = Array.from(container.querySelectorAll("button")).find((item) => item.textContent === "Reset filters");
    await act(() => reset?.click());
    expect(container.querySelectorAll(".model-card")).toHaveLength(2);
  });

  it("offers a retry inside the detail dialog after a failed pull", async () => {
    const adapter: ModelHubAdapter = {
      listModels: vi.fn().mockResolvedValue([model]),
      pullModel: vi.fn().mockRejectedValue(new Error("Connection lost")),
      cancelPull: vi.fn(), removeModel: vi.fn(),
    };
    container = document.createElement("div"); document.body.append(container);
    root = createRoot(container);
    await act(async () => root?.render(createElement(ModelsScreen, { adapter, machine })));
    await act(() => container?.querySelector<HTMLButtonElement>(".model-card__body")?.click());
    await act(async () => container?.querySelector<HTMLButtonElement>(".model-detail__actions button")?.click());
    expect(container.querySelector("[role=dialog]")?.textContent).toContain("Connection lost");
    expect(container.querySelector(".model-detail__actions button")?.textContent).toBe("Retry download");
    await act(async () => container?.querySelector<HTMLButtonElement>(".model-detail__actions button")?.click());
    expect(adapter.pullModel).toHaveBeenCalledTimes(2);
  });
  it("loads real adapter records and starts a cancellable pull", async () => {
    let pullOptions: PullOptions | undefined;
    const adapter: ModelHubAdapter = {
      listModels: vi.fn().mockResolvedValue([model]),
      pullModel: vi.fn((_id, options) => {
        pullOptions = options;
        return new Promise<ModelRecord>(() => undefined);
      }),
      cancelPull: vi.fn().mockResolvedValue(undefined),
      removeModel: vi.fn().mockResolvedValue(undefined),
    };
    container = document.createElement("div");
    document.body.append(container);
    root = createRoot(container);

    await act(async () => {
      root?.render(createElement(ModelsScreen, { adapter, machine }));
    });
    expect(container.textContent).toContain("Pearl Chat 8B");
    const pull = Array.from(container.querySelectorAll("button")).find(
      (button) => button.textContent === "Pull model",
    );
    await act(() => pull?.click());
    expect(adapter.pullModel).toHaveBeenCalledWith(
      "pearl-chat",
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );

    await act(() =>
      pullOptions?.onProgress({
        receivedBytes: 3 * gib,
        totalBytes: 6 * gib,
      }),
    );
    expect(container.textContent).toContain("Downloading 50%");

    const cancel = Array.from(container.querySelectorAll("button")).find(
      (button) => button.textContent === "Cancel",
    );
    await act(async () => cancel?.click());
    expect(adapter.cancelPull).toHaveBeenCalledWith("pearl-chat");
    expect(pullOptions?.signal.aborted).toBe(true);
  });

  it("shows an accessible empty state after a search misses", async () => {
    const adapter: ModelHubAdapter = {
      listModels: vi.fn().mockResolvedValue([]),
      pullModel: vi.fn(),
      cancelPull: vi.fn(),
      removeModel: vi.fn(),
    };
    container = document.createElement("div");
    document.body.append(container);
    root = createRoot(container);
    await act(async () => {
      root?.render(createElement(ModelsScreen, { adapter, machine }));
    });
    expect(container.textContent).toContain("No models match those filters.");
    expect(
      Array.from(container.querySelectorAll("button")).some(
        (button) => button.textContent === "Clear filters",
      ),
    ).toBe(true);
  });

  it("keeps the removal dialog open and reports backend rejection", async () => {
    const installed = { ...model, installed: true };
    const adapter: ModelHubAdapter = {
      listModels: vi.fn().mockResolvedValue([installed]),
      pullModel: vi.fn(),
      cancelPull: vi.fn(),
      removeModel: vi.fn().mockRejectedValue(new Error("model is serving")),
    };
    container = document.createElement("div");
    document.body.append(container);
    root = createRoot(container);
    await act(async () => root?.render(createElement(ModelsScreen, { adapter, machine })));
    const manage = Array.from(container.querySelectorAll("button")).find(
      (button) => button.textContent === "Manage",
    );
    await act(() => manage?.click());
    const remove = Array.from(container.querySelectorAll("button")).find(
      (button) => button.textContent === "Remove model",
    );
    await act(() => remove?.click());
    const confirm = Array.from(container.querySelectorAll("button")).find(
      (button) => button.textContent === "Remove local files",
    );
    await act(async () => confirm?.click());
    expect(container.textContent).toContain("model is serving");
    expect(container.querySelector("[role=alertdialog]")?.textContent).toContain("model is serving");
    expect(container.textContent).not.toContain("Could not load the catalog");
    expect(container.textContent).toContain("Remove local files");
  });
});
