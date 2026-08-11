import { describe, expect, it, vi } from "vitest";
import type { TapiocaDesktopApi } from "../shared/ipc";
import { createRendererAdapters } from "./adapters";

function apiFixture(overrides: Partial<TapiocaDesktopApi> = {}) {
  const base: TapiocaDesktopApi = {
    health: vi.fn().mockResolvedValue({
      status: "ready",
      protocolVersion: 1,
      platform: "darwin",
      arch: "arm64",
      checkedAt: "2026-08-01T12:00:00Z",
    }),
    systemSnapshot: vi.fn().mockResolvedValue({
      platform: "macos",
      arch: "arm64",
      cpuCount: 10,
      accelerators: ["apple", "cpu"],
      memoryBytes: 32 * 1024 ** 3,
      modelsPath: "/models",
      modelsBytes: 0,
      availableDiskBytes: 100 * 1024 ** 3,
    }),
    models: vi.fn().mockResolvedValue({ catalog: [], installed: [] }),
    modelPull: vi.fn(),
    modelRemove: vi.fn(),
    cancelJob: vi.fn().mockResolvedValue(true),
    chatStatus: vi.fn().mockResolvedValue({ installed: [], servers: [] }),
    serverStart: vi.fn(),
    serverStop: vi.fn(),
    serverStatus: vi.fn().mockResolvedValue([]),
    chatRequest: vi.fn(),
    selectWorkspace: vi.fn().mockResolvedValue(undefined),
    agentDescribe: vi.fn(),
    agentLaunch: vi.fn(),
    creatorCapabilities: vi.fn().mockResolvedValue({
      image: { available: true, method: "image.generate" },
      video: { available: true, method: "video.generate" },
      speech: { available: false, method: "speech.generate", error_code: "runtime_adapter_required" },
      voice_clone: { available: false, method: "voice.clone", error_code: "runtime_adapter_required" },
      outputs: { binary_in_protocol: false, managed_local_paths: true },
    }),
    creatorCatalog: vi.fn().mockResolvedValue([]),
    creatorPickFile: vi.fn().mockResolvedValue(undefined),
    creatorSaveRecording: vi.fn(),
    creatorGenerate: vi.fn(),
    creatorOutputs: vi.fn().mockResolvedValue([]),
    creatorLoraList: vi.fn().mockResolvedValue([]),
    creatorLoraInspect: vi.fn().mockResolvedValue({}),
    creatorReveal: vi.fn(),
    creatorSaveMetadata: vi.fn().mockResolvedValue(true),
    onJobEvent: vi.fn(() => () => undefined),
  };
  return Object.assign(base, overrides);
}

describe("renderer adapters", () => {
  it("treats an online sidecar with no model server as startable and filters non-chat models", async () => {
    const api = apiFixture({
      chatStatus: vi.fn().mockResolvedValue({
        installed: [
          { name: "chat", repo: "a/chat", kind: "text", backend: "llama.cpp" },
          { name: "image", repo: "a/image", kind: "image", backend: "diffusers" },
          { name: "video", repo: "a/video", kind: "video", backend: "diffusers-video" },
          { name: "voice", repo: "a/voice", kind: "speech", backend: "speech" },
        ],
        servers: [],
      }),
    });
    const adapters = createRendererAdapters(api, vi.fn());
    await expect(adapters.chat.connection()).resolves.toBe("startable");
    await expect(adapters.chat.models()).resolves.toEqual([
      expect.objectContaining({ id: "chat", ready: true }),
    ]);
  });

  it("reports unavailable speech from runtime capabilities", async () => {
    const adapters = createRendererAdapters(apiFixture(), vi.fn());
    await expect(adapters.creator.models("speech")).resolves.toEqual([
      expect.objectContaining({
        ready: false,
        detail: "Unavailable: runtime_adapter_required",
      }),
    ]);
  });

  it("exposes the installed MiniMax-H3 CUDA model on an NVIDIA Windows machine", async () => {
    const h3 = {
      name: "minimax-h3:fl2va-int8-cuda",
      repo: "Comfy-Org/MiniMax-H3",
      kind: "video",
      backend: "comfy-h3-cuda",
      size: "~41 GiB",
      memory: "32 GiB system RAM; 16 GiB VRAM recommended",
      platforms: ["windows", "linux"],
      operation: "video.generate",
      supports_input_image: true,
      requires_input_image: false,
      supports_lora: true,
      available: true,
      width: 864,
      height: 480,
      steps: 20,
      frames: 73,
      fps: 24,
    };
    const api = apiFixture({
      systemSnapshot: vi.fn().mockResolvedValue({
        platform: "windows",
        arch: "amd64",
        cpuCount: 16,
        accelerators: ["nvidia", "cpu"],
        memoryBytes: 64 * 1024 ** 3,
        modelsPath: "C:\\Users\\test\\.tapioca\\models",
        modelsBytes: 41 * 1024 ** 3,
        availableDiskBytes: 100 * 1024 ** 3,
      }),
      creatorCatalog: vi.fn().mockResolvedValue([h3]),
      models: vi.fn().mockResolvedValue({
        catalog: [h3],
        installed: [{
          name: h3.name,
          repo: h3.repo,
          kind: h3.kind,
          backend: h3.backend,
        }],
      }),
    });
    const adapters = createRendererAdapters(api, vi.fn());

    await expect(adapters.creator.models("video")).resolves.toEqual([
      expect.objectContaining({
        id: h3.name,
        ready: true,
        supportsInputImage: true,
        supportsLoRA: true,
      }),
    ]);
    await expect(adapters.machine()).resolves.toEqual(
      expect.objectContaining({ platform: "windows", accelerators: ["nvidia", "cpu"] }),
    );
  });

  it("turns installed adapter files into assignable LoRA references", async () => {
    const api = apiFixture({
      creatorLoraList: vi.fn().mockResolvedValue([
        {
          id: "lora-1",
          reference: "civitai://2830065/3193337#motion.safetensors",
          provider: "civitai",
          file: "motion.safetensors",
          bytes: 256 * 1024 ** 2,
          bases: ["minimax-h3"],
        },
        {
          id: "lora-2",
          reference: "local://flux-style#style.safetensors",
          provider: "local",
          file: "style.safetensors",
          bytes: 128 * 1024 ** 2,
          bases: ["flux"],
        },
      ]),
    });
    const adapters = createRendererAdapters(api, vi.fn());

    await expect(
      adapters.creator.availableLoras("minimax-h3:fl2va-int8-cuda"),
    ).resolves.toEqual([
      expect.objectContaining({
        reference: "civitai://2830065/3193337#motion.safetensors",
        compatible: true,
      }),
      expect.objectContaining({
        reference: "local://flux-style#style.safetensors",
        compatible: false,
      }),
    ]);
  });

  it("builds Home from validated runtime facts without fake activity", async () => {
    const api = apiFixture();
    const adapters = createRendererAdapters(api, vi.fn());
    const home = await adapters.home.getSnapshot();
    expect(home.hardware).toMatchObject({
      platform: "macos",
      memoryBytes: 32 * 1024 ** 3,
      accelerator: "Apple Silicon",
    });
    expect(home.recentActivity).toEqual([]);
    expect(home.readiness[1]).toMatchObject({
      state: "current",
      destination: "models",
    });
  });

  it("correlates model progress by job id and returns refreshed installed state", async () => {
    let eventListener:
      | Parameters<TapiocaDesktopApi["onJobEvent"]>[0]
      | undefined;
    const catalog = {
      name: "gemma",
      kind: "text",
      backend: "mlx",
      repo: "creator/gemma",
      size: "6 GiB",
      memory: "12 GiB",
      platforms: ["macos"],
    };
    const models = vi
      .fn()
      .mockResolvedValueOnce({ catalog: [catalog], installed: [] })
      .mockResolvedValueOnce({
        catalog: [catalog],
        installed: [
          {
            name: "gemma",
            repo: "creator/gemma",
            kind: "text",
            backend: "mlx",
          },
        ],
      });
    const api = apiFixture({
      serverStatus: vi.fn().mockResolvedValue([
        {
          id: "server-gemma",
          model: "gemma",
          endpoint: "http://127.0.0.1:11435",
          state: "running",
        },
      ]),
      models,
      onJobEvent: vi.fn((listener) => {
        eventListener = listener;
        return () => undefined;
      }),
      modelPull: vi.fn(async ({ jobId }) => {
        eventListener?.({
          version: 1,
          type: "event",
          event: "job.progress",
          job_id: jobId,
          sequence: 2,
          timestamp: "2026-08-01T12:00:00Z",
          data: { bytes: 3, total_bytes: 6 },
        });
        return {
          name: "gemma",
          repo: "creator/gemma",
          kind: "text",
          backend: "mlx",
        };
      }),
    });
    const adapters = createRendererAdapters(api, vi.fn());
    await adapters.models.listModels();
    const progress = vi.fn();
    const installed = await adapters.models.pullModel("gemma", {
      signal: new AbortController().signal,
      onProgress: progress,
    });
    expect(progress).toHaveBeenCalledWith({
      receivedBytes: 3,
      totalBytes: 6,
    });
    expect(installed.installed).toBe(true);
  });

  it("correlates chat job events and exposes one backend completion", async () => {
    let listener: Parameters<TapiocaDesktopApi["onJobEvent"]>[0] | undefined;
    const api = apiFixture({
      serverStatus: vi.fn().mockResolvedValue([
        {
          id: "server-gemma",
          model: "gemma",
          endpoint: "http://127.0.0.1:11435",
          state: "running",
        },
      ]),
      onJobEvent: vi.fn((next) => {
        listener = next;
        return () => undefined;
      }),
      chatRequest: vi.fn(async ({ jobId }) => {
        listener?.({
          version: 1,
          type: "event",
          event: "job.log",
          job_id: jobId,
          sequence: 2,
          timestamp: "2026-08-01T12:00:00Z",
          data: { message: "Working locally" },
        });
        return {
          id: "response-1",
          object: "chat.completion",
          created: 1,
          model: "gemma",
          choices: [
            {
              index: 0,
              message: {
                role: "assistant",
                content: "Hello",
                tool_calls: [{
                  id: "call-1",
                  type: "function" as const,
                  function: { name: "read_file", arguments: "{\"path\":\"README.md\"}" },
                }],
              },
              finish_reason: "stop",
            },
          ],
        };
      }),
    });
    const adapters = createRendererAdapters(api, vi.fn());
    const events = vi.fn();
    await adapters.chat.send(
      {
        modelId: "gemma",
        messages: [{ role: "user", content: "Hi" }],
      },
      events,
    );
    expect(events.mock.calls.map(([event]) => event.type)).toEqual([
      "thinking.delta",
      "tool",
      "content.delta",
      "completed",
    ]);
  });

  it("starts an installed model server on loopback before chat", async () => {
    const serverStatus = vi
      .fn()
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([{
        id: "desktop-chat",
        model: "gemma",
        endpoint: "http://127.0.0.1:11435",
        state: "running",
      }]);
    const api = apiFixture({
      serverStatus,
      serverStart: vi.fn().mockResolvedValue({
        id: "desktop-chat",
        model: "gemma",
        endpoint: "http://127.0.0.1:11435",
        state: "starting",
      }),
      chatRequest: vi.fn().mockResolvedValue({
        id: "response-2",
        object: "chat.completion",
        created: 1,
        model: "gemma",
        choices: [{ index: 0, message: { role: "assistant", content: "Ready" }, finish_reason: "stop" }],
      }),
    });
    const adapters = createRendererAdapters(api, vi.fn());
    await adapters.chat.send(
      { modelId: "gemma", messages: [{ role: "user", content: "Hi" }] },
      vi.fn(),
    );
    expect(api.serverStart).toHaveBeenCalledWith(expect.objectContaining({
      model: "gemma",
      jobId: expect.stringMatching(/^server-start-/),
    }));
    expect(api.chatRequest).toHaveBeenCalled();
  });

  it("cancels pending startup and waits for the server to stop", async () => {
    let state: "missing" | "starting" | "stopped" = "missing";
    let serverId = "";
    const api = apiFixture({
      serverStatus: vi.fn(async ({ id } = {}) => {
        if (state === "missing") return [];
        return [{
          id: id ?? serverId,
          model: "gemma",
          endpoint: "http://127.0.0.1:11435",
          state,
        }];
      }),
      serverStart: vi.fn(async ({ id }) => {
        serverId = id;
        state = "starting";
        return { id, model: "gemma", endpoint: "http://127.0.0.1:11435", state };
      }),
      serverStop: vi.fn(async ({ id }) => {
        state = "stopped";
        return { id, model: "gemma", endpoint: "http://127.0.0.1:11435", state: "stopping" as const };
      }),
    });
    const adapters = createRendererAdapters(api, vi.fn());
    const sending = adapters.chat.send(
      { modelId: "gemma", messages: [{ role: "user", content: "Hi" }] },
      vi.fn(),
    );
    await vi.waitFor(() => expect(api.serverStart).toHaveBeenCalled());
    await adapters.chat.stop();
    await expect(sending).rejects.toMatchObject({ name: "AbortError" });
    expect(api.cancelJob).toHaveBeenCalledWith({
      jobId: expect.stringMatching(/^server-start-/),
    });
    expect(api.cancelJob).toHaveBeenCalledWith({
      jobId: expect.stringMatching(/^chat-/),
    });
    expect(api.serverStop).toHaveBeenCalledWith(expect.objectContaining({ id: serverId }));
    await expect(api.serverStatus({ id: serverId })).resolves.toEqual([
      expect.objectContaining({ state: "stopped" }),
    ]);
  });

  it("fully stops the old model before starting a switched model", async () => {
    let current: { id: string; model: string; state: "running" | "stopped" } = {
      id: "server-a",
      model: "model-a",
      state: "running",
    };
    const order: string[] = [];
    const api = apiFixture({
      serverStatus: vi.fn(async ({ id } = {}) =>
        !id || id === current.id
          ? [{ ...current, endpoint: "http://127.0.0.1:11435" }]
          : [],
      ),
      serverStop: vi.fn(async ({ id }) => {
        order.push(`stop:${current.model}`);
        current = { ...current, id, state: "stopped" };
        return { ...current, endpoint: "http://127.0.0.1:11435", state: "stopping" as const };
      }),
      serverStart: vi.fn(async ({ id, model }) => {
        order.push(`start:${model}`);
        current = { id, model, state: "running" };
        return { ...current, endpoint: "http://127.0.0.1:11435", state: "starting" as const };
      }),
      chatRequest: vi.fn().mockResolvedValue({
        id: "response", object: "chat.completion", created: 1, model: "model",
        choices: [{ index: 0, message: { role: "assistant", content: "ok" }, finish_reason: "stop" }],
      }),
    });
    const adapters = createRendererAdapters(api, vi.fn());
    await adapters.chat.send(
      { modelId: "model-a", messages: [{ role: "user", content: "A" }] },
      vi.fn(),
    );
    await adapters.chat.send(
      { modelId: "model-b", messages: [{ role: "user", content: "B" }] },
      vi.fn(),
    );
    expect(order).toEqual(["stop:model-a", "start:model-b"]);
  });

  it("reports local API readiness and launches agents in an external terminal", async () => {
    const api = apiFixture({
      agentLaunch: vi.fn().mockResolvedValue({
        runId: "3f853258-e5a6-46dd-b625-d0da26a3aab8",
        message: "codex opened in a terminal window.",
      }),
    });
    const adapters = createRendererAdapters(api, vi.fn());
    await expect(adapters.agents.readiness()).resolves.toMatchObject({
      server: "ready",
      endpoint: "http://127.0.0.1:11435/v1",
    });
    const onEvent = vi.fn();
    await expect(
      adapters.agents.launch(
        {
          agent: "codex",
          modelId: "gemma",
          workspace: { path: "/workspace", displayName: "workspace" },
        },
        onEvent,
      ),
    ).resolves.toEqual({ runId: "3f853258-e5a6-46dd-b625-d0da26a3aab8" });
    expect(api.agentLaunch).toHaveBeenCalledWith(expect.objectContaining({
      agent: "codex",
      model: "gemma",
    }));
    expect(onEvent).toHaveBeenCalledWith({ type: "exit", code: 0 });
  });
});
