import type { TapiocaDesktopApi } from "../shared/ipc";
import type {
  HomeAdapter,
  HomeNavigation,
  HomeSnapshot,
} from "../features/home";
import type {
  Accelerator,
  MachineProfile,
  ModelHubAdapter,
  ModelKind,
  ModelRecord,
} from "../features/models";
import type {
  ChatAdapter,
  ChatMessage,
  ChatSession,
  ChatSessionSummary,
  ChatStreamEvent,
} from "../features/chat";
import type {
  AgentAdapter,
  AgentDefinition,
  AgentEnvironmentEntry,
  AgentLaunchRequest,
} from "../features/agents";
import type {
  CreatorAdapter,
  CreatorMode,
  CreatorProgressEvent,
  CreatorRequest,
} from "../features/create";

const gib = 1024 ** 3;

export interface RendererAdapters {
  home: HomeAdapter;
  homeNavigation: HomeNavigation;
  models: ModelHubAdapter;
  machine(): Promise<MachineProfile>;
  chat: ChatAdapter;
  agents: AgentAdapter;
  creator: CreatorAdapter;
}

export function createRendererAdapters(
  api: TapiocaDesktopApi,
  navigate: (destination: string) => void,
): RendererAdapters {
  const sessions = new Map<string, ChatSession>();
  const activeChatJobs = new Set<string>();
  let activeChatServer: { id: string; model: string } | undefined;
  let activeServerStartJob: string | undefined;
  let serverTransition: Promise<void> = Promise.resolve();
  let transitionAbort: AbortController | undefined;
  const activePullJobs = new Map<string, string>();

  const machine = async (): Promise<MachineProfile> => {
    const system = await api.systemSnapshot();
    return {
      platform: system.platform,
      memoryBytes: system.memoryBytes,
      availableDiskBytes: system.availableDiskBytes,
      accelerators: system.accelerators as Accelerator[],
    };
  };

  const listModels = async (): Promise<ModelRecord[]> => {
    const result = await api.models();
    const installed = new Map(result.installed.map((model) => [model.name, model]));
    return result.catalog.map((model) => {
      const local = installed.get(model.name);
      const requirements = modelRequirements(model.backend, model.size, model.memory);
      return {
        id: model.name,
        name: model.name,
        creator: model.repo.split("/")[0] || model.repo,
        description:
          [model.features, model.languages, model.gpu].filter(Boolean).join(" · ") ||
          `${model.kind} model using ${model.backend}`,
        kind: desktopKind(model.kind),
        backend: model.backend,
        tags: [
          model.backend,
          model.repo,
          ...(model.platforms ?? []),
          ...(model.languages ? [model.languages] : []),
        ],
        requirements: {
          ...requirements,
          platforms: model.platforms.length
            ? (model.platforms as MachineProfile["platform"][])
            : ["macos", "windows", "linux"],
        },
        installed: Boolean(local),
        installedBytes: local ? requirements.diskBytes : undefined,
      };
    });
  };

  const modelAdapter: ModelHubAdapter = {
    listModels,
    async pullModel(modelId, options) {
      const jobId = `pull-${crypto.randomUUID()}`;
      activePullJobs.set(modelId, jobId);
      const unsubscribe = api.onJobEvent((event) => {
        if (event.job_id !== jobId || event.event !== "job.progress") return;
        const progress = pullProgress(event.data);
        if (progress) options.onProgress(progress);
      });
      const abort = () => void api.cancelJob({ jobId }).catch(() => undefined);
      options.signal.addEventListener("abort", abort, { once: true });
      try {
        await api.modelPull({ name: modelId, jobId });
        const refreshed = await listModels();
        const installed = refreshed.find((model) => model.id === modelId);
        if (!installed) throw new Error("Pulled model is missing from registry");
        return installed;
      } finally {
        activePullJobs.delete(modelId);
        options.signal.removeEventListener("abort", abort);
        unsubscribe();
      }
    },
    async cancelPull(modelId) {
      const jobId = activePullJobs.get(modelId);
      if (jobId) await api.cancelJob({ jobId });
    },
    async removeModel(modelId) {
      await api.modelRemove({ name: modelId, confirm: modelId });
    },
  };

  const chatAdapter: ChatAdapter = {
    async connection() {
      await api.health();
      const status = await api.chatStatus();
      const running = status.servers.find(({ state }) => state === "running");
      if (running) {
        activeChatServer = { id: running.id, model: running.model };
        return "ready";
      }
      return "startable";
    },
    async reconnect() {
      return this.connection();
    },
    async models() {
      const status = await api.chatStatus();
      return status.installed
        .filter((model) => isChatModelKind(model.kind))
        .map((model) => ({
        id: model.name,
        name: model.name,
        detail: model.backend,
        ready: true,
        }));
    },
    async sessions(): Promise<ChatSessionSummary[]> {
      return [...sessions.values()]
        .map(({ id, title, messages }) => ({
          id,
          title,
          updatedAt:
            messages.at(-1)?.createdAt ?? new Date(0).toISOString(),
        }))
        .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt));
    },
    async loadSession(id) {
      const session = sessions.get(id);
      if (!session) throw new Error("Conversation is no longer available");
      return structuredClone(session);
    },
    async send(request, onEvent) {
      const sessionId = request.sessionId ?? crypto.randomUUID();
      const jobId = `chat-${crypto.randomUUID()}`;
      activeChatJobs.add(jobId);
      const unsubscribe = api.onJobEvent((event) => {
        if (event.job_id !== jobId) return;
        if (event.event === "job.log") {
          const message = eventDataMessage(event.data);
          if (message) onEvent({ type: "thinking.delta", text: `${message}\n` });
        }
        if (event.event === "job.failed") {
          onEvent({
            type: "error",
            message: eventDataMessage(event.data) ?? "Chat job failed",
            retryable: false,
          });
        }
      });
      try {
        await transitionToChatServer(request.modelId);
        const response = await api.chatRequest({
          jobId,
          model: request.modelId,
          messages: request.messages,
        });
        const message = response.choices[0]?.message;
        const content = textContent(message?.content);
        if (message?.reasoning_content) {
          onEvent({ type: "thinking.delta", text: message.reasoning_content });
        }
        for (const toolCall of message?.tool_calls ?? []) {
          onEvent({
            type: "tool",
            tool: {
              id: toolCall.id,
              name: toolCall.function.name,
              status: "requested",
              summary: `Model requested this tool; it was not executed. Arguments: ${toolCall.function.arguments.slice(0, 2_000)}`,
            },
          });
        }
        if (content) onEvent({ type: "content.delta", text: content });
        onEvent({ type: "completed" });
        const assistantMessageId = response.id || crypto.randomUUID();
        const previous = sessions.get(sessionId);
        const messages: ChatMessage[] = [
          ...request.messages.map((item, index) => ({
            id: `${sessionId}-${index}`,
            role: item.role,
            content: item.content,
            createdAt: new Date().toISOString(),
          })),
          {
            id: assistantMessageId,
            role: "assistant",
            content,
            thinking: message?.reasoning_content,
            tools: message?.tool_calls?.map((toolCall) => ({
              id: toolCall.id,
              name: toolCall.function.name,
              status: "requested" as const,
              summary: `Model requested this tool; it was not executed. Arguments: ${toolCall.function.arguments.slice(0, 2_000)}`,
            })),
            createdAt: new Date().toISOString(),
          },
        ];
        sessions.set(sessionId, {
          id: sessionId,
          title: previous?.title ?? request.messages[0]?.content.slice(0, 48) ?? "Chat",
          modelId: request.modelId,
          messages,
        });
        return { sessionId, assistantMessageId };
      } finally {
        activeChatJobs.delete(jobId);
        unsubscribe();
      }
    },
    async stop() {
      transitionAbort?.abort();
      const cancellations: Promise<unknown>[] = [];
      if (activeServerStartJob) {
        cancellations.push(
          api.cancelJob({ jobId: activeServerStartJob }).catch(() => false),
        );
      }
      for (const jobId of activeChatJobs) {
        cancellations.push(api.cancelJob({ jobId }).catch(() => false));
      }
      await Promise.all(cancellations);
      activeChatJobs.clear();
      const shutdown = serverTransition
        .catch(() => undefined)
        .then(async () => {
          await stopActiveChatServer();
        });
      serverTransition = shutdown;
      await shutdown;
    },
  };

  function transitionToChatServer(model: string): Promise<void> {
    if (activeChatServer && activeChatServer.model !== model) {
      transitionAbort?.abort();
    }
    const abort = new AbortController();
    transitionAbort = abort;
    const transition = serverTransition
      .catch(() => undefined)
      .then(async () => {
        await ensureChatServer(model, abort.signal);
      });
    serverTransition = transition.then(
      () => undefined,
      () => undefined,
    );
    return transition.finally(() => {
      if (transitionAbort === abort) transitionAbort = undefined;
    });
  }

  async function ensureChatServer(model: string, signal: AbortSignal) {
    throwIfAborted(signal);
    const statuses = await api.serverStatus({});
    const ready = statuses.find(
      (server) => server.model === model && server.state === "running",
    );
    if (ready) {
      activeChatServer = { id: ready.id, model };
      return;
    }
    for (const server of statuses) {
      if (
        server.model !== model &&
        (server.state === "running" ||
          server.state === "starting" ||
          server.state === "stopping")
      ) {
        await stopServerAndWait(server.id, signal);
      }
    }
    throwIfAborted(signal);
    const id = `desktop-chat-${crypto.randomUUID()}`;
    const jobId = `server-start-${crypto.randomUUID()}`;
    activeChatServer = { id, model };
    activeServerStartJob = jobId;
    try {
      await api.serverStart({ id, model, jobId });
      for (let attempt = 0; attempt < 120; attempt += 1) {
        throwIfAborted(signal);
        const status = (await api.serverStatus({ id }))[0];
        if (status?.state === "running") return;
        if (status?.state === "failed" || status?.state === "stopped") {
          throw new Error(status.error || `Could not start ${model}`);
        }
        await abortableDelay(250, signal);
      }
      throw new Error(`Timed out while starting ${model}`);
    } finally {
      if (activeServerStartJob === jobId) activeServerStartJob = undefined;
    }
  }

  async function stopActiveChatServer() {
    const active = activeChatServer;
    if (!active) return;
    await stopServerAndWait(active.id);
    if (activeChatServer?.id === active.id) activeChatServer = undefined;
  }

  async function stopServerAndWait(id: string, signal?: AbortSignal) {
    const current = (await api.serverStatus({ id }))[0];
    if (!current || current.state === "stopped") return;
    if (current.state !== "stopping") {
      await api.serverStop({
        id,
        jobId: `server-stop-${crypto.randomUUID()}`,
      });
    }
    for (let attempt = 0; attempt < 120; attempt += 1) {
      throwIfAborted(signal);
      const status = (await api.serverStatus({ id }))[0];
      if (!status || status.state === "stopped") return;
      await abortableDelay(250, signal);
    }
    throw new Error(`Timed out while stopping model server ${id}`);
  }

  const agentDefinitions: Array<[AgentDefinition["id"], string]> = [
    ["codex", "Codex"],
    ["claude", "Claude Code"],
    ["opencode", "OpenCode"],
    ["openclaw", "OpenClaw"],
    ["hermes", "Hermes"],
  ];

  const agentAdapter: AgentAdapter = {
    async definitions() {
      return Promise.all(agentDefinitions.map(async ([id, name]) => {
        const descriptor = await api.agentDescribe({ agent: id, model: "local-model" });
        return {
          id,
          name,
          description: descriptor.installed
            ? `Ready at ${descriptor.executable}`
            : `${name} is not installed or is not on PATH.`,
          installed: descriptor.installed,
        };
      }));
    },
    async models() {
      const status = await chatAdapter.models();
      return status.map((model) => ({
        id: model.id,
        name: model.name,
        ready: model.ready,
        context: model.detail,
      }));
    },
    async readiness() {
      const health = await api.health();
      return health.status === "ready"
        ? { server: "ready", endpoint: "http://127.0.0.1:11435/v1" }
        : { server: "offline", message: "The local control service is unavailable." };
    },
    async chooseWorkspace() {
      return api.selectWorkspace();
    },
    async environment(request: AgentLaunchRequest): Promise<AgentEnvironmentEntry[]> {
      const descriptor = await api.agentDescribe({
        agent: request.agent,
        model: request.modelId,
      });
      return [
        { name: "Executable", value: descriptor.executable },
        { name: "Endpoint", value: descriptor.endpoint },
        { name: "Protocol", value: descriptor.protocol },
        ...Object.entries(descriptor.environment).map(([name, value]) => ({
          name,
          value,
          sensitive: name.includes("TOKEN") || name.includes("KEY"),
        })),
      ];
    },
    async launch(request, onEvent) {
      onEvent({ type: "status", status: "launching", message: "Opening a terminal…" });
      const result = await api.agentLaunch({
        agent: request.agent,
        model: request.modelId,
        workspace: request.workspace,
      });
      onEvent({ type: "log", level: "info", message: result.message });
      onEvent({ type: "exit", code: 0 });
      return { runId: result.runId };
    },
    async stop() {
      throw new Error("The agent is running in its terminal window; close that window to stop it.");
    },
  };

  const creatorAdapter: CreatorAdapter = {
    outputs() {
      return api.creatorOutputs();
    },
    async availableLoras(modelId) {
      const baseFamily = loraFamily(modelId);
      return (await api.creatorLoraList()).flatMap((row) => {
        const segments = row.file.split("/").filter(Boolean);
        if (segments.length < 3) return [];
        const [owner, repository, ...fileParts] = segments;
        const reference = `hf://${owner}/${repository}#${fileParts.join("/")}`;
        const adapterFamily = loraFamily(row.file);
        const compatible = !baseFamily || !adapterFamily || baseFamily === adapterFamily;
        return [{
          reference,
          name: fileParts.at(-1) ?? row.file,
          bytes: row.bytes,
          compatible,
          ...(!compatible
            ? { reason: `Looks like ${adapterFamily}, but the selected model is ${baseFamily}.` }
            : adapterFamily
              ? { reason: `Matches the ${adapterFamily} model family.` }
              : { reason: "Tapioca will validate compatibility before generation." }),
        }];
      });
    },
    async models(mode) {
      const [capabilities, catalog, installed] = await Promise.all([
        api.creatorCapabilities(),
        api.creatorCatalog(),
        api.models(),
      ]);
      const capability =
        mode === "voice-clone"
          ? capabilities.voice_clone
          : capabilities[mode];
      const installedNames = new Set(installed.installed.map(({ name }) => name));
      const kind = mode === "voice-clone" ? "speech" : mode;
      const matches = catalog.filter((model) => model.kind === kind);
      if (!matches.length && !capability.available) {
        return [{
          id: `${mode}-unavailable`,
          name: mode === "voice-clone" ? "Voice cloning" : "Speech",
          modes: [mode],
          ready: false,
          detail: `Unavailable: ${capability.error_code ?? "runtime unavailable"}`,
        }];
      }
      return matches.map((model) => ({
        id: model.name,
        name: model.name,
        modes: [mode],
        ready: capability.available && model.available && installedNames.has(model.name),
        detail: !capability.available
          ? `Unavailable: ${capability.error_code ?? "runtime unavailable"}`
          : !model.available
            ? `Unavailable: ${model.unavailable_reason ?? "runtime unavailable"}`
            : installedNames.has(model.name)
              ? model.backend
              : `Install from Models · ${model.backend}`,
        limits: { maxWidth: 4096, maxHeight: 4096, maxFrames: 513 },
        supportsInputImage: model.supports_input_image,
        supportsLoRA: model.supports_lora,
        requiresInputImage: model.requires_input_image,
        requiresVoiceReference:
          kind === "speech" &&
          (model.name.includes("qwen3-tts") || model.name.includes("chatterbox:nano")),
        defaults: {
          ...(model.width ? { width: model.width } : {}),
          ...(model.height ? { height: model.height } : {}),
          ...(model.steps ? { steps: model.steps } : {}),
          ...(model.frames ? { frames: model.frames } : {}),
          ...(model.fps ? { fps: model.fps } : {}),
        },
      }));
    },
    pickFile(kind) {
      return api.creatorPickFile({ kind });
    },
    saveVoiceRecording(bytes, durationSeconds) {
      return api.creatorSaveRecording({ bytes, durationSeconds });
    },
    async generate(request, onEvent) {
      const jobId = `creator-${crypto.randomUUID()}`;
      const unsubscribe = api.onJobEvent((event) => {
        if (event.job_id !== jobId) return;
        const message = eventDataMessage(event.data);
        if (event.event === "job.started" || event.event === "job.log") {
          onEvent({ type: "queued", message });
        } else if (event.event === "job.progress") {
          const progress = eventProgress(event.data);
          if (progress !== undefined) onEvent({ type: "progress", progress, message });
          else onEvent({ type: "queued", message });
        } else if (event.event === "job.failed") {
          onEvent({ type: "error", message: message ?? "Generation failed", retryable: false });
        }
      });
      void (async () => {
        try {
        for (const lora of request.loras) {
          if (lora.source.type === "huggingface") {
            await api.creatorLoraInspect({ reference: lora.source.reference });
          }
        }
        const output = await api.creatorGenerate({
          jobId,
          mode: request.mode,
          model: request.modelId,
          prompt: request.prompt,
          text: request.text,
          inputImageToken: request.inputImage?.token,
          voiceReferenceToken: request.voiceReference?.token,
          loras: request.loras.map((lora) =>
            lora.source.type === "huggingface"
              ? { type: "huggingface" as const, reference: lora.source.reference, weight: lora.weight }
              : { type: "local" as const, token: lora.source.file.token, weight: lora.weight },
          ),
          settings: request.settings,
        });
        onEvent({ type: "completed", output });
        } catch (cause) {
          onEvent({
            type: "error",
            message: cause instanceof Error ? cause.message : String(cause),
            retryable: false,
          });
        } finally {
          unsubscribe();
        }
      })();
      return { jobId };
    },
    async cancel(jobId) {
      await api.cancelJob({ jobId });
    },
    async reveal(outputId) {
      await api.creatorReveal({ outputId });
    },
    async saveMetadata(outputId) {
      await api.creatorSaveMetadata({ outputId });
    },
  };

  const homeAdapter: HomeAdapter = {
    async getSnapshot(signal): Promise<HomeSnapshot> {
      const [health, system, models] = await Promise.all([
        api.health(),
        api.systemSnapshot(),
        api.models(),
      ]);
      if (signal?.aborted) throw new DOMException("Aborted", "AbortError");
      return {
        readiness: [
          {
            id: "runtime",
            title: "Local runtime",
            description: health.status === "ready" ? "Connected and private" : health.status,
            state: health.status === "ready" ? "complete" : "blocked",
          },
          {
            id: "model",
            title: "Installed model",
            description: models.installed.length
              ? `${models.installed.length} available locally`
              : "Pull a model to begin",
            state: models.installed.length ? "complete" : "current",
            actionLabel: models.installed.length ? undefined : "Browse",
            destination: models.installed.length ? undefined : "models",
          },
        ],
        hardware: {
          platform: system.platform,
          processor: `${system.arch} · ${system.cpuCount} CPU cores`,
          memoryBytes: system.memoryBytes,
          accelerator: system.accelerators
            .filter((accelerator) => accelerator !== "cpu")
            .map((accelerator) => ({
              apple: "Apple Silicon",
              nvidia: "NVIDIA GPU",
              amd: "AMD GPU",
              intel: "Intel GPU",
            })[accelerator])
            .filter(Boolean)
            .join(" + ") || undefined,
        },
        storage: {
          modelsBytes: system.modelsBytes,
          availableBytes: system.availableDiskBytes,
          location: system.modelsPath,
        },
        recentActivity: [],
      };
    },
  };

  return {
    home: homeAdapter,
    homeNavigation: { open: navigate },
    models: modelAdapter,
    machine,
    chat: chatAdapter,
    agents: agentAdapter,
    creator: creatorAdapter,
  };
}

function eventProgress(data: unknown): number | undefined {
  if (!data || typeof data !== "object") return undefined;
  const value = data as Record<string, unknown>;
  if (typeof value.progress === "number") return Math.max(0, Math.min(1, value.progress));
  if (typeof value.current === "number" && typeof value.total === "number" && value.total > 0) {
    return Math.max(0, Math.min(1, value.current / value.total));
  }
  return undefined;
}

function desktopKind(kind: string): ModelKind {
  if (kind === "image") return "image";
  if (kind === "video") return "video";
  if (["speech", "tts", "audio", "voice"].includes(kind)) return "speech";
  return "chat";
}

function modelRequirements(backend: string, size?: string, memory?: string) {
  const lower = backend.toLowerCase();
  const accelerators: Accelerator[] = lower.includes("mlx") || lower.includes("comfy-h3-mps")
    ? ["apple"]
    : lower.includes("cuda") || lower.includes("comfy-h3-cuda")
      ? ["nvidia"]
      : ["cpu", "apple", "nvidia", "amd", "intel"];
  return {
    memoryBytes: parseByteEstimate(memory) ?? Number.MAX_SAFE_INTEGER,
    recommendedMemoryBytes: parseRecommendedMemory(memory),
    diskBytes: parseByteEstimate(size) ?? Number.MAX_SAFE_INTEGER,
    accelerators,
  };
}

function parseByteEstimate(value?: string): number | undefined {
  if (!value) return undefined;
  const match = value.match(/(\d+(?:\.\d+)?)\s*(GiB|GB|MiB|MB)/i);
  if (!match) return undefined;
  const amount = Number(match[1]);
  const unit = match[2].toLowerCase();
  return Math.round(
    amount *
      (unit === "gib"
        ? gib
        : unit === "gb"
          ? 1e9
          : unit === "mib"
            ? 1024 ** 2
            : 1e6),
  );
}

function parseRecommendedMemory(value?: string): number | undefined {
  if (!value) return undefined;
  const match = value.match(/(\d+(?:\.\d+)?)\s*GiB\s*recommended/i);
  return match ? Number(match[1]) * gib : undefined;
}

function pullProgress(data: unknown) {
  if (!data || typeof data !== "object") return undefined;
  const value = data as Record<string, unknown>;
  if (typeof value.bytes !== "number" || typeof value.total_bytes !== "number") {
    return undefined;
  }
  return { receivedBytes: value.bytes, totalBytes: value.total_bytes };
}

function eventDataMessage(data: unknown): string | undefined {
  if (!data || typeof data !== "object") return undefined;
  const message = (data as Record<string, unknown>).message;
  return typeof message === "string" ? message : undefined;
}

function textContent(content: unknown): string {
  if (typeof content === "string") return content;
  if (Array.isArray(content)) {
    return content
      .map((item) =>
        item && typeof item === "object" && "text" in item
          ? String((item as { text: unknown }).text)
          : "",
      )
      .join("");
  }
  return content == null ? "" : String(content);
}

function isChatModelKind(kind: string): boolean {
  return ["chat", "text", "llm"].includes(kind.trim().toLowerCase());
}

function loraFamily(value: string): string | undefined {
  const normalized = value.toLowerCase();
  for (const [needle, family] of [
    ["minimax-h3", "MiniMax-H3"],
    ["minimax_h3", "MiniMax-H3"],
    ["flux", "Flux"],
    ["qwen", "Qwen"],
    ["wan", "Wan"],
    ["ltx", "LTX"],
  ] as const) {
    if (normalized.includes(needle)) return family;
  }
  return undefined;
}

function throwIfAborted(signal?: AbortSignal): void {
  if (signal?.aborted) throw new DOMException("Chat server transition cancelled", "AbortError");
}

function abortableDelay(milliseconds: number, signal?: AbortSignal): Promise<void> {
  if (!signal) return new Promise((resolve) => setTimeout(resolve, milliseconds));
  throwIfAborted(signal);
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      signal.removeEventListener("abort", onAbort);
      resolve();
    }, milliseconds);
    const onAbort = () => {
      clearTimeout(timer);
      reject(new DOMException("Chat server transition cancelled", "AbortError"));
    };
    signal.addEventListener("abort", onAbort, { once: true });
  });
}
