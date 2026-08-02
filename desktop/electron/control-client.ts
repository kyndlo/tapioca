import { spawn as nodeSpawn } from "node:child_process";
import { EventEmitter } from "node:events";
import path from "node:path";
import {
  BoundedNdjsonParser,
  controlRequestSchema,
  handshakeResultSchema,
  SIDECAR_PROTOCOL_VERSION,
  type ControlEvent,
  type ControlMethod,
  type ControlResponse,
} from "../src/shared/sidecar";

const MAX_STDERR_BYTES = 64 * 1024;

export type ControlClientState =
  | { status: "stopped" }
  | { status: "starting" }
  | { status: "ready"; serverVersion: string }
  | { status: "stopping" }
  | { status: "crashed"; reason: string };

export interface WritablePipe {
  write(data: string): boolean;
  end(): void;
}

export interface ControlProcess extends EventEmitter {
  pid?: number;
  kill?(signal?: NodeJS.Signals): boolean;
  stdin: WritablePipe;
  stdout: EventEmitter;
  stderr: EventEmitter;
}

export interface SpawnOptions {
  shell: false;
  stdio: ["pipe", "pipe", "pipe"];
  windowsHide: true;
  detached: boolean;
}

export type SpawnControlProcess = (
  executable: string,
  args: readonly string[],
  options: SpawnOptions,
) => ControlProcess;

export interface TreeKillStrategy {
  terminate(process: ControlProcess, force: boolean): Promise<boolean>;
}

export type SpawnTerminationCommand = (
  command: string,
  args: readonly string[],
  options: {
    shell: false;
    windowsHide: true;
    stdio: "ignore";
  },
) => EventEmitter;

export interface ControlClientOptions {
  platform?: NodeJS.Platform;
  spawn?: SpawnControlProcess;
  treeKill?: TreeKillStrategy;
  requestTimeoutMs?: number;
  gracefulShutdownMs?: number;
  forcedShutdownMs?: number;
  maxLineBytes?: number;
  idFactory?: () => string;
}

export class ControlRequestError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly retryable: boolean,
    readonly details?: unknown,
  ) {
    super(message);
    this.name = "ControlRequestError";
  }
}

interface PendingRequest {
  resolve(value: unknown): void;
  reject(error: Error): void;
  timer?: ReturnType<typeof setTimeout>;
}

export interface SidecarLocationContext {
  isPackaged: boolean;
  resourcesPath: string;
  appPath: string;
  platform: NodeJS.Platform;
}

export function resolveSidecarExecutable(
  context: SidecarLocationContext,
): string {
  const pathApi = context.platform === "win32" ? path.win32 : path;
  const executable =
    context.platform === "win32"
      ? "tapioca-control.exe"
      : "tapioca-control";
  return context.isPackaged
    ? pathApi.resolve(context.resourcesPath, "sidecar", executable)
    : pathApi.resolve(context.appPath, "..", "bin", executable);
}

export function resolveCliExecutable(context: SidecarLocationContext): string {
  const pathApi = context.platform === "win32" ? path.win32 : path;
  const executable = context.platform === "win32" ? "tapioca.exe" : "tapioca";
  return context.isPackaged
    ? pathApi.resolve(context.resourcesPath, "sidecar", executable)
    : pathApi.resolve(context.appPath, "..", "bin", executable);
}

export function createPlatformTreeKillStrategy(
  platform: NodeJS.Platform,
  spawnCommand: SpawnTerminationCommand = (command, args, options) =>
    nodeSpawn(command, [...args], options),
): TreeKillStrategy {
  return {
    async terminate(process, force) {
      if (!process.pid) return false;
      const signal: NodeJS.Signals = force ? "SIGKILL" : "SIGTERM";
      if (platform === "win32") {
        const args = [
          "/PID",
          String(process.pid),
          "/T",
          ...(force ? ["/F"] : []),
        ];
        return await new Promise<boolean>((resolve) => {
          const taskkill = spawnCommand("taskkill", args, {
            shell: false,
            windowsHide: true,
            stdio: "ignore",
          });
          taskkill.once("error", () => resolve(false));
          taskkill.once("exit", (code: number | null) => resolve(code === 0));
        });
      }
      try {
        globalThis.process.kill(-process.pid, signal);
        return true;
      } catch {
        try {
          globalThis.process.kill(process.pid, signal);
          return true;
        } catch {
          return false;
        }
      }
    },
  };
}

const defaultSpawn: SpawnControlProcess = (executable, args, options) =>
  nodeSpawn(executable, [...args], options) as unknown as ControlProcess;

export class ControlClient {
  private readonly parser: BoundedNdjsonParser;
  private readonly pending = new Map<string, PendingRequest>();
  private readonly eventListeners = new Set<(event: ControlEvent) => void>();
  private readonly spawnProcess: SpawnControlProcess;
  private readonly treeKill: TreeKillStrategy;
  private readonly platform: NodeJS.Platform;
  private readonly requestTimeoutMs: number;
  private readonly gracefulShutdownMs: number;
  private readonly forcedShutdownMs: number;
  private readonly idFactory: () => string;
  private child: ControlProcess | undefined;
  private nextId = 0;
  private stderrBuffer = "";
  private currentState: ControlClientState = { status: "stopped" };

  constructor(options: ControlClientOptions = {}) {
    this.platform = options.platform ?? process.platform;
    this.spawnProcess = options.spawn ?? defaultSpawn;
    this.treeKill =
      options.treeKill ?? createPlatformTreeKillStrategy(this.platform);
    this.requestTimeoutMs = options.requestTimeoutMs ?? 15_000;
    this.gracefulShutdownMs = options.gracefulShutdownMs ?? 2_000;
    this.forcedShutdownMs = options.forcedShutdownMs ?? 1_000;
    this.parser = new BoundedNdjsonParser(options.maxLineBytes);
    this.idFactory =
      options.idFactory ?? (() => `desktop-${Date.now()}-${++this.nextId}`);
  }

  get state(): ControlClientState {
    return this.currentState;
  }

  get stderrDiagnostics(): string {
    return this.stderrBuffer;
  }

  async start(executable: string): Promise<void> {
    if (this.child || this.currentState.status === "starting") {
      throw new Error("Tapioca control sidecar is already running");
    }
    this.currentState = { status: "starting" };

    let child: ControlProcess;
    try {
      child = this.spawnProcess(executable, [], {
        shell: false,
        stdio: ["pipe", "pipe", "pipe"],
        windowsHide: true,
        detached: this.platform !== "win32",
      });
    } catch (error) {
      this.crash(`Unable to spawn control sidecar: ${String(error)}`);
      throw error;
    }
    this.child = child;
    this.attach(child);

    try {
      const result = await this.requestInternal("handshake");
      const handshake = handshakeResultSchema.parse(result);
      if (this.child !== child) throw new Error("Control sidecar exited");
      this.currentState = {
        status: "ready",
        serverVersion: handshake.name,
      };
    } catch (error) {
      this.crash(`Handshake failed: ${errorMessage(error)}`);
      void this.treeKill.terminate(child, true);
      throw error;
    }
  }

  request(
    method: Exclude<ControlMethod, "handshake">,
    params?: unknown,
    jobId?: string,
    timeoutMs: number | null = this.requestTimeoutMs,
  ): Promise<unknown> {
    if (this.currentState.status !== "ready") {
      return Promise.reject(new Error("Tapioca control sidecar is not ready"));
    }
    return this.requestInternal(method, params, jobId, timeoutMs);
  }

  onEvent(listener: (event: ControlEvent) => void): () => void {
    this.eventListeners.add(listener);
    return () => this.eventListeners.delete(listener);
  }

  async stop(): Promise<void> {
    const child = this.child;
    if (!child) {
      this.currentState = { status: "stopped" };
      return;
    }
    this.currentState = { status: "stopping" };
    child.stdin.end();

    if (!(await this.waitForExit(child, this.gracefulShutdownMs))) {
      await this.treeKill.terminate(child, false);
      if (!(await this.waitForExit(child, this.forcedShutdownMs))) {
        await this.treeKill.terminate(child, true);
        if (!(await this.waitForExit(child, this.forcedShutdownMs))) {
          const error = new Error(
            "Control sidecar termination could not be confirmed",
          );
          this.currentState = { status: "crashed", reason: error.message };
          this.rejectPending(error);
          throw error;
        }
      }
    }
    this.rejectPending(new Error("Tapioca control sidecar stopped"));
    this.currentState = { status: "stopped" };
  }

  private requestInternal(
    method: ControlMethod,
    params?: unknown,
    jobId?: string,
    timeoutMs: number | null = this.requestTimeoutMs,
  ): Promise<unknown> {
    const child = this.child;
    if (!child) return Promise.reject(new Error("Control sidecar is unavailable"));
    const id = this.idFactory();
    const request = controlRequestSchema.parse({
      version: SIDECAR_PROTOCOL_VERSION,
      type: "request",
      id,
      method,
      ...(params === undefined ? {} : { params }),
      ...(jobId === undefined ? {} : { job_id: jobId }),
    });

    return new Promise((resolve, reject) => {
      const timer =
        timeoutMs === null
          ? undefined
          : setTimeout(() => {
              this.pending.delete(id);
              reject(new Error(`Control request timed out: ${method}`));
            }, timeoutMs);
      this.pending.set(id, { resolve, reject, timer });
      try {
        child.stdin.write(`${JSON.stringify(request)}\n`);
      } catch (error) {
        if (timer) clearTimeout(timer);
        this.pending.delete(id);
        reject(error instanceof Error ? error : new Error(String(error)));
      }
    });
  }

  private attach(child: ControlProcess): void {
    child.stdout.on("data", (chunk: Buffer | string) => {
      if (this.child !== child) return;
      try {
        for (const envelope of this.parser.push(chunk)) {
          if (envelope.type === "response") this.handleResponse(envelope);
          else for (const listener of this.eventListeners) listener(envelope);
        }
      } catch (error) {
        this.crash(`Invalid sidecar output: ${errorMessage(error)}`);
        void this.treeKill.terminate(child, true);
      }
    });
    child.stdout.once("end", () => {
      if (this.child === child && this.currentState.status !== "stopping") {
        this.crash("Control sidecar closed stdout");
      }
    });
    child.stderr.on("data", (chunk: Buffer | string) => {
      this.stderrBuffer = `${this.stderrBuffer}${chunk.toString()}`.slice(
        -MAX_STDERR_BYTES,
      );
    });
    child.once(
      "exit",
      (code: number | null, signal: NodeJS.Signals | null) => {
        if (this.child !== child) return;
        this.child = undefined;
        if (this.currentState.status === "stopping") return;
        this.crash(`Control sidecar exited (${code ?? signal ?? "unknown"})`);
      },
    );
    child.once("error", (error: Error) => {
      if (this.child !== child) return;
      this.child = undefined;
      this.crash(`Control sidecar error: ${error.message}`);
    });
  }

  private handleResponse(response: ControlResponse): void {
    const pending = this.pending.get(response.id);
    if (!pending) return;
    this.pending.delete(response.id);
    if (pending.timer) clearTimeout(pending.timer);
    if (response.error) {
      pending.reject(
        new ControlRequestError(
          response.error.code,
          response.error.message,
          response.error.retryable,
          response.error.details,
        ),
      );
    } else {
      pending.resolve(response.result);
    }
  }

  private crash(reason: string): void {
    this.currentState = { status: "crashed", reason };
    this.rejectPending(new Error(reason));
  }

  private rejectPending(error: Error): void {
    for (const pending of this.pending.values()) {
      if (pending.timer) clearTimeout(pending.timer);
      pending.reject(error);
    }
    this.pending.clear();
  }

  private waitForExit(child: ControlProcess, timeoutMs: number): Promise<boolean> {
    if (this.child !== child) return Promise.resolve(true);
    return new Promise((resolve) => {
      const timer = setTimeout(() => {
        child.removeListener("exit", onExit);
        resolve(false);
      }, timeoutMs);
      const onExit = () => {
        clearTimeout(timer);
        resolve(true);
      };
      child.once("exit", onExit);
    });
  }
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
