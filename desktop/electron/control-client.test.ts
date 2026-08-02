import { EventEmitter } from "node:events";
import { describe, expect, it, vi } from "vitest";
import {
  ControlClient,
  createPlatformTreeKillStrategy,
  type ControlProcess,
  resolveSidecarExecutable,
  resolveCliExecutable,
  type SpawnControlProcess,
  type TreeKillStrategy,
} from "./control-client";

class FakeProcess extends EventEmitter implements ControlProcess {
  pid = 4242;
  stdout = new EventEmitter();
  stderr = new EventEmitter();
  writes: string[] = [];
  ended = false;
  onWrite?: (line: string) => void;
  onEnd?: () => void;
  stdin = {
    write: (line: string) => {
      this.writes.push(line);
      this.onWrite?.(line);
      return true;
    },
    end: () => {
      this.ended = true;
      this.onEnd?.();
    },
  };
}

function response(id: string, result: unknown): string {
  return `${JSON.stringify({ version: 1, type: "response", id, result })}\n`;
}

function createHarness(options: {
  handshake?: unknown;
  maxLineBytes?: number;
}) {
  const process = new FakeProcess();
  let counter = 0;
  process.onWrite = (line) => {
    const request = JSON.parse(line);
    if (request.method === "handshake") {
      queueMicrotask(() =>
        process.stdout.emit(
          "data",
          response(
            request.id,
            options.handshake ?? {
              protocol_version: 1,
              name: "tapioca-control",
              capabilities: {
                methods: [
                  "handshake",
                  "capabilities.get",
                  "health.get",
                  "catalog.list",
                  "installed.list",
                  "job.cancel",
                ],
                events: ["job.started", "job.completed", "job.failed"],
                max_request_bytes: 4194304,
                max_concurrency: 8,
              },
            },
          ),
        ),
      );
    }
  };
  const spawn = vi.fn(() => process) as unknown as SpawnControlProcess;
  const treeKill: TreeKillStrategy = {
    terminate: vi.fn().mockResolvedValue(true),
  };
  const client = new ControlClient({
    spawn,
    treeKill,
    maxLineBytes: options.maxLineBytes,
    requestTimeoutMs: 100,
    idFactory: () => `id-${++counter}`,
  });
  return { client, process, spawn, treeKill };
}

describe("sidecar path resolution", () => {
  it("supports development, packaged, Windows, and paths with spaces", () => {
    expect(
      resolveSidecarExecutable({
        isPackaged: false,
        resourcesPath: "/unused",
        appPath: "/repo/desktop",
        platform: "darwin",
      }),
    ).toBe("/repo/bin/tapioca-control");
    expect(
      resolveSidecarExecutable({
        isPackaged: true,
        resourcesPath: "C:\\Program Files\\Tapioca\\resources",
        appPath: "C:\\unused",
        platform: "win32",
      }),
    ).toContain("tapioca-control.exe");
    expect(
      resolveSidecarExecutable({
        isPackaged: true,
        resourcesPath: "/Applications/Tapioca Local AI/resources",
        appPath: "/unused",
        platform: "darwin",
      }),
    ).toBe(
      "/Applications/Tapioca Local AI/resources/sidecar/tapioca-control",
    );
    expect(resolveCliExecutable({
      isPackaged: false,
      resourcesPath: "/unused",
      appPath: "/repo/desktop",
      platform: "darwin",
    })).toBe("/repo/bin/tapioca");
    expect(resolveCliExecutable({
      isPackaged: true,
      resourcesPath: "C:\\Program Files\\Tapioca\\resources",
      appPath: "C:\\unused",
      platform: "win32",
    })).toContain("sidecar\\tapioca.exe");
  });
});

describe("ControlClient transport", () => {
  it("spawns without a shell and becomes ready only after handshake", async () => {
    const { client, spawn } = createHarness({});
    const started = client.start("/path with spaces/tapioca-control");
    expect(client.state.status).toBe("starting");
    await started;
    expect(client.state).toEqual({
      status: "ready",
      serverVersion: "tapioca-control",
    });
    expect(spawn).toHaveBeenCalledWith(
      "/path with spaces/tapioca-control",
      [],
      expect.objectContaining({ shell: false }),
    );
  });

  it("rejects a mismatched handshake", async () => {
    const { client } = createHarness({
      handshake: {
        protocol_version: 2,
        name: "tapioca-control",
        capabilities: {
          methods: [],
          events: [],
          max_request_bytes: 4194304,
          max_concurrency: 1,
        },
      },
    });
    await expect(client.start("/bin/control")).rejects.toThrow();
    expect(client.state.status).toBe("crashed");
  });

  it("correlates out-of-order responses and routes events", async () => {
    const { client, process } = createHarness({});
    await client.start("/bin/control");
    const events = vi.fn();
    client.onEvent(events);
    const first = client.request("health.get");
    const second = client.request("catalog.list");
    const firstRequest = JSON.parse(process.writes[1]);
    const secondRequest = JSON.parse(process.writes[2]);
    process.stdout.emit(
      "data",
      response(secondRequest.id, { models: [] }) +
        JSON.stringify({
          version: 1,
          type: "event",
          event: "job.started",
          job_id: "job-1",
          sequence: 1,
          timestamp: "2026-08-01T12:00:00.000Z",
          data: { progress: 0.5 },
        }) +
        "\n",
    );
    process.stdout.emit("data", response(firstRequest.id, { status: "ready" }));
    await expect(second).resolves.toEqual({ models: [] });
    await expect(first).resolves.toEqual({ status: "ready" });
    expect(events).toHaveBeenCalledWith(
      expect.objectContaining({ event: "job.started", sequence: 1 }),
    );
  });

  it("accepts fragmented output and continuously bounds stderr", async () => {
    const { client, process } = createHarness({});
    await client.start("/bin/control");
    const pending = client.request("installed.list");
    const id = JSON.parse(process.writes[1]).id;
    const line = response(id, { models: [] });
    process.stdout.emit("data", line.slice(0, 9));
    process.stderr.emit("data", "x".repeat(70 * 1024));
    process.stdout.emit("data", line.slice(9));
    await expect(pending).resolves.toEqual({ models: [] });
    expect(client.stderrDiagnostics.length).toBe(64 * 1024);
  });

  it("allows explicitly unbounded requests for long local generation jobs", async () => {
    vi.useFakeTimers();
    try {
      const { client, process } = createHarness({});
      await client.start("/bin/control");
      const pending = client.request("image.generate", { prompt: "fox" }, "job-image", null);
      const id = JSON.parse(process.writes[1]).id;
      await vi.advanceTimersByTimeAsync(1_000);
      process.stdout.emit("data", response(id, { path: "/outputs/fox.png" }));
      await expect(pending).resolves.toEqual({ path: "/outputs/fox.png" });
    } finally {
      vi.useRealTimers();
    }
  });

  it("crashes and rejects requests on malformed output, overflow, EOF, or exit", async () => {
    for (const trigger of ["malformed", "overflow", "eof", "exit"] as const) {
      const { client, process } = createHarness({ maxLineBytes: 1024 });
      await client.start("/bin/control");
      const pending = client.request("health.get");
      if (trigger === "malformed") process.stdout.emit("data", "{bad}\n");
      if (trigger === "overflow") process.stdout.emit("data", "x".repeat(1025));
      if (trigger === "eof") process.stdout.emit("end");
      if (trigger === "exit") process.emit("exit", 2, null);
      await expect(pending).rejects.toThrow();
      expect(client.state.status).toBe("crashed");
    }
  });

  it("reports a wrong or missing binary", async () => {
    const spawn = vi.fn(() => {
      throw new Error("ENOENT");
    }) as unknown as SpawnControlProcess;
    const client = new ControlClient({ spawn });
    await expect(client.start("/missing/tapioca-control")).rejects.toThrow(
      "ENOENT",
    );
    expect(client.state.status).toBe("crashed");
  });

  it("shuts down gracefully by closing stdin and awaiting exit", async () => {
    const { client, process, treeKill } = createHarness({});
    await client.start("/bin/control");
    process.onEnd = () => queueMicrotask(() => process.emit("exit", 0, null));
    await client.stop();
    expect(process.ended).toBe(true);
    expect(treeKill.terminate).not.toHaveBeenCalled();
    expect(client.state.status).toBe("stopped");
  });

  it("uses bounded gentle then forced tree termination", async () => {
    const { process, spawn } = createHarness({});
    const terminations: boolean[] = [];
    const treeKill: TreeKillStrategy = {
      async terminate(child, force) {
        terminations.push(force);
        if (force) queueMicrotask(() => child.emit("exit", null, "SIGKILL"));
        return true;
      },
    };
    const client = new ControlClient({
      spawn,
      treeKill,
      gracefulShutdownMs: 1,
      forcedShutdownMs: 1,
      idFactory: (() => {
        let id = 0;
        return () => `force-${++id}`;
      })(),
    });
    await client.start("/bin/control");
    await client.stop();
    expect(process.ended).toBe(true);
    expect(terminations).toEqual([false, true]);
    expect(client.state.status).toBe("stopped");
  });

  it("does not report stopped when forced termination never produces exit", async () => {
    const { process, spawn } = createHarness({});
    const treeKill: TreeKillStrategy = {
      terminate: vi.fn().mockResolvedValue(false),
    };
    const client = new ControlClient({
      spawn,
      treeKill,
      gracefulShutdownMs: 1,
      forcedShutdownMs: 1,
    });
    await client.start("/bin/control");
    await expect(client.stop()).rejects.toThrow("could not be confirmed");
    expect(process.ended).toBe(true);
    expect(client.state.status).toBe("crashed");
  });
});

describe("Windows process-tree strategy", () => {
  it("uses taskkill with fixed PID/tree arguments and force only when requested", async () => {
    const calls: Array<{ command: string; args: readonly string[]; options: unknown }> = [];
    const spawnCommand = vi.fn((command, args, options) => {
      calls.push({ command, args, options });
      const child = new EventEmitter();
      queueMicrotask(() => child.emit("exit", 0));
      return child;
    });
    const strategy = createPlatformTreeKillStrategy("win32", spawnCommand);
    const process = new FakeProcess();

    await expect(strategy.terminate(process, false)).resolves.toBe(true);
    await expect(strategy.terminate(process, true)).resolves.toBe(true);

    expect(calls[0]).toEqual({
      command: "taskkill",
      args: ["/PID", "4242", "/T"],
      options: { shell: false, windowsHide: true, stdio: "ignore" },
    });
    expect(calls[1].args).toEqual(["/PID", "4242", "/T", "/F"]);
  });

  it("reports taskkill command failure", async () => {
    const spawnCommand = vi.fn(() => {
      const child = new EventEmitter();
      queueMicrotask(() => child.emit("exit", 1));
      return child;
    });
    const strategy = createPlatformTreeKillStrategy("win32", spawnCommand);
    await expect(strategy.terminate(new FakeProcess(), true)).resolves.toBe(
      false,
    );
  });
});
