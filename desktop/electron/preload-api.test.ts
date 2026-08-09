import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it, vi } from "vitest";
import { IPC_CHANNELS } from "../src/shared/ipc";
import { createTapiocaApi, type PreloadTransport } from "./preload-api";

function transportWith(
  invoke: PreloadTransport["invoke"],
): PreloadTransport {
  return {
    invoke,
    on: vi.fn(),
    removeListener: vi.fn(),
  };
}

describe("preload allowlist", () => {
  it("exposes only the intended methods", () => {
    const api = createTapiocaApi(transportWith(vi.fn()));
    expect(Object.keys(api).sort()).toEqual(
      [
        "agentDescribe",
        "agentLaunch",
        "cancelJob",
        "chatRequest",
        "chatStatus",
        "serverStart",
        "serverStatus",
        "serverStop",
        "creatorCapabilities",
        "creatorCatalog",
        "creatorGenerate",
        "creatorLoraInspect",
        "creatorLoraList",
        "creatorOutputs",
        "creatorPickFile",
        "creatorReveal",
        "creatorSaveMetadata",
        "creatorSaveRecording",
        "health",
        "modelPull",
        "modelRemove",
        "models",
        "onJobEvent",
        "selectWorkspace",
        "systemSnapshot",
      ].sort(),
    );
    expect(Object.isFrozen(api)).toBe(true);
  });

  it("invokes only the fixed health channel and validates output", async () => {
    const invoke = vi.fn().mockResolvedValue({
      status: "ready",
      protocolVersion: 1,
      platform: "darwin",
      arch: "arm64",
      checkedAt: "2026-08-01T12:00:00Z",
    });
    const api = createTapiocaApi(transportWith(invoke));

    await expect(api.health()).resolves.toMatchObject({ status: "ready" });
    expect(invoke).toHaveBeenCalledWith(IPC_CHANNELS.health);
  });

  it("rejects invalid model pull input before IPC", async () => {
    const invoke = vi.fn();
    const api = createTapiocaApi(transportWith(invoke));

    await expect(
      api.modelPull({ name: "", jobId: "" }),
    ).rejects.toThrow();
    expect(invoke).not.toHaveBeenCalled();
  });

  it("validates job events and returns a working unsubscribe", () => {
    const on = vi.fn();
    const removeListener = vi.fn();
    const transport = {
      invoke: vi.fn(),
      on,
      removeListener,
    };
    const api = createTapiocaApi(transport);
    const listener = vi.fn();
    const unsubscribe = api.onJobEvent(listener);
    const wrapped = on.mock.calls[0][1];

    const goldenEvent = JSON.parse(
      readFileSync(
        path.resolve(process.cwd(), "../contracts/control/v1/events.ndjson"),
        "utf8",
      ).split("\n")[0],
    );
    wrapped(undefined, goldenEvent);
    expect(listener).toHaveBeenCalledOnce();

    unsubscribe();
    expect(removeListener).toHaveBeenCalledWith(
      IPC_CHANNELS.jobEvent,
      wrapped,
    );
  });
});
