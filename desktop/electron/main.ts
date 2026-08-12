import {
  app,
  BrowserWindow,
  dialog,
  ipcMain,
  net,
  protocol,
  session,
  shell,
  type IpcMainInvokeEvent,
  type OpenDialogOptions,
} from "electron";
import { fileURLToPath, pathToFileURL } from "node:url";
import path from "node:path";
import os from "node:os";
import { lstat, mkdir, readdir, statfs, writeFile } from "node:fs/promises";
import { spawn } from "node:child_process";
import { statSync } from "node:fs";
import { z } from "zod";
import {
  installedListResultSchema,
  installedModelSchema,
} from "../src/shared/sidecar";
import {
  IPC_CHANNELS,
  ipcSchemas,
} from "../src/shared/ipc";
import {
  ControlClient,
  resolveCliExecutable,
  resolveSidecarExecutable,
} from "./control-client";
import { ControlService } from "./control-service";
import { isAudioCapturePermission, isTrustedRendererUrl } from "./security";
import { settleControlBeforeWindow } from "./startup";
import { MediaRegistry, type MediaKind } from "./media-registry";

protocol.registerSchemesAsPrivileged([
  {
    scheme: "tapioca-media",
    privileges: { standard: true, secure: true, supportFetchAPI: true, stream: true },
  },
]);

const currentDirectory = path.dirname(fileURLToPath(import.meta.url));
const isDevelopment = Boolean(process.env.VITE_DEV_SERVER_URL);
const trustedWebContents = new Set<number>();
const rendererIndexPath = path.resolve(
  currentDirectory,
  "../dist/renderer/index.html",
);
const packagedRendererUrl = pathToFileURL(rendererIndexPath).href;
const controlClient = new ControlClient();
const controlService = new ControlService(controlClient);
const mediaRegistry = new MediaRegistry();
let shutdownStarted = false;
let sidecarExecutable: string | undefined;
let restartPromise: Promise<void> | undefined;
const zSystemInfo = z
  .object({
    goos: z.string().min(1),
    goarch: z.string().min(1),
    cpu_count: z.number().int().positive(),
    accelerators: z.array(z.enum(["apple", "nvidia", "amd", "intel", "cpu"])),
    protocol_version: z.literal(1),
  })
  .strict();
const zStorageInfo = z
  .object({
    home: z.string().min(1),
    models_path: z.string().min(1),
    models_bytes: z.number().int().nonnegative(),
  })
  .strict();
const zCancelResult = z
  .object({ job_id: z.string().min(1), cancelled: z.boolean() })
  .strict();
const zCreatorOutput = z.object({
  path: z.string().min(1),
  kind: z.string().min(1),
  mime: z.string().min(1),
  bytes: z.number().int().nonnegative(),
  model: z.string().min(1),
  created_at: z.string().datetime(),
}).strict();

function publicInstalledModel(
  model: z.infer<typeof installedModelSchema>,
) {
  return {
    name: model.name,
    repo: model.repo,
    ...(model.filename ? { filename: model.filename } : {}),
    kind: model.kind,
    backend: model.backend,
  };
}

function assertTrustedFrame(event: IpcMainInvokeEvent): void {
  const frame = event.senderFrame;
  if (
    !trustedWebContents.has(event.sender.id) ||
    !frame ||
    frame !== event.sender.mainFrame
  ) {
    throw new Error("Untrusted IPC sender");
  }

  const trusted = isTrustedRendererUrl(frame.url, {
    developmentServerUrl: isDevelopment
      ? process.env.VITE_DEV_SERVER_URL
      : undefined,
    packagedRendererUrl,
  });
  if (!trusted) {
    throw new Error("Untrusted IPC origin");
  }
}

function registerContentSecurityPolicy(): void {
  session.defaultSession.setPermissionRequestHandler(
    (webContents, permission, callback, details) => {
      const mediaTypes = "mediaTypes" in details ? details.mediaTypes : undefined;
      callback(
        trustedWebContents.has(webContents.id) &&
        isTrustedRendererUrl(webContents.getURL(), {
          developmentServerUrl: isDevelopment
            ? process.env.VITE_DEV_SERVER_URL
            : undefined,
          packagedRendererUrl,
        }) &&
        isAudioCapturePermission(permission, mediaTypes),
      );
    },
  );
  session.defaultSession.setPermissionCheckHandler(
    (webContents, permission, _origin, details) =>
      Boolean(
        webContents &&
        trustedWebContents.has(webContents.id) &&
        isTrustedRendererUrl(webContents.getURL(), {
          developmentServerUrl: isDevelopment
            ? process.env.VITE_DEV_SERVER_URL
            : undefined,
          packagedRendererUrl,
        }) &&
        isAudioCapturePermission(
          permission,
          details.mediaType ? [details.mediaType] : undefined,
        ),
      ),
  );
  session.defaultSession.webRequest.onHeadersReceived((details, callback) => {
    const connectSource = isDevelopment
      ? "connect-src 'self' http://127.0.0.1:5173 ws://127.0.0.1:5173"
      : "connect-src 'self'";
    const scriptSource = isDevelopment
      ? "script-src 'self' 'unsafe-inline'"
      : "script-src 'self'";
    const policy = [
      "default-src 'self'",
      scriptSource,
      "style-src 'self' 'unsafe-inline'",
      "img-src 'self' data: blob: tapioca-media:",
      "font-src 'self'",
      connectSource,
      "media-src 'self' blob: tapioca-media:",
      "object-src 'none'",
      "base-uri 'self'",
      "frame-ancestors 'none'",
      "form-action 'none'",
    ].join("; ");

    callback({
      responseHeaders: {
        ...details.responseHeaders,
        "Content-Security-Policy": [policy],
      },
    });
  });
}

function registerIpcHandlers(): void {
  ipcMain.handle(IPC_CHANNELS.health, async (event) => {
    assertTrustedFrame(event);
    if (controlClient.state.status !== "ready" && sidecarExecutable) {
      restartPromise ??= controlClient.start(sidecarExecutable).finally(() => {
        restartPromise = undefined;
      });
      await restartPromise;
    }
    return ipcSchemas.healthResult.parse(await controlService.health());
  });

  ipcMain.handle(IPC_CHANNELS.systemSnapshot, async (event) => {
    assertTrustedFrame(event);
    const [system, storage] = await Promise.all([
      controlClient.request("system.info"),
      controlClient.request("storage.info"),
    ]);
    const parsedSystem = zSystemInfo.parse(system);
    const parsedStorage = zStorageInfo.parse(storage);
    const disk = await statfs(parsedStorage.models_path).catch(() =>
      statfs(parsedStorage.home),
    );
    return ipcSchemas.systemSnapshotResult.parse({
      platform:
        parsedSystem.goos === "darwin"
          ? "macos"
          : parsedSystem.goos === "windows"
            ? "windows"
            : "linux",
      arch: parsedSystem.goarch,
      cpuCount: parsedSystem.cpu_count,
      accelerators: parsedSystem.accelerators,
      memoryBytes: os.totalmem(),
      modelsPath: parsedStorage.models_path,
      modelsBytes: parsedStorage.models_bytes,
      availableDiskBytes: disk.bavail * disk.bsize,
    });
  });

  ipcMain.handle(IPC_CHANNELS.models, async (event) => {
    assertTrustedFrame(event);
    const [catalog, installed] = await Promise.all([
      controlClient.request("catalog.list"),
      controlClient.request("installed.list"),
    ]);
    return ipcSchemas.modelsResult.parse({
      catalog,
      installed: installedListResultSchema
        .parse(installed)
        .map(publicInstalledModel),
    });
  });
  ipcMain.handle(IPC_CHANNELS.modelPull, async (event, raw: unknown) => {
    assertTrustedFrame(event);
    const input = ipcSchemas.modelPullInput.parse(raw);
    return ipcSchemas.modelPullResult.parse(
      publicInstalledModel(
        installedModelSchema.parse(await controlClient.request(
        "model.pull",
		{
			name: input.name,
			accept_license: input.acceptLicense,
			hf_token: input.accessToken,
		},
        input.jobId,
        null,
        )),
      ),
    );
  });
  ipcMain.handle(IPC_CHANNELS.modelRemove, async (event, raw: unknown) => {
    assertTrustedFrame(event);
    const input = ipcSchemas.modelRemoveInput.parse(raw);
    if (input.confirm !== input.name) throw new Error("Confirmation mismatch");
    await controlClient.request("model.remove", {
      name: input.name,
      dry_run: false,
      confirm: input.confirm,
    });
  });
  ipcMain.handle(IPC_CHANNELS.cancelJob, async (event, raw: unknown) => {
    assertTrustedFrame(event);
    const input = ipcSchemas.cancelJobInput.parse(raw);
    const result = zCancelResult.parse(
      await controlClient.request("job.cancel", { job_id: input.jobId }),
    );
    return result.cancelled;
  });
  ipcMain.handle(IPC_CHANNELS.chatStatus, async (event) => {
    assertTrustedFrame(event);
    const [installed, servers] = await Promise.all([
      controlClient.request("installed.list"),
      controlClient.request("server.status"),
    ]);
    return ipcSchemas.chatStatusResult.parse({
      installed: installedListResultSchema
        .parse(installed)
        .map(publicInstalledModel),
      servers,
    });
  });
  ipcMain.handle(IPC_CHANNELS.serverStart, async (event, raw: unknown) => {
    assertTrustedFrame(event);
    const input = ipcSchemas.serverStartInput.parse(raw);
    return ipcSchemas.serverMutationResult.parse(await controlClient.request(
      "server.start",
      { id: input.id, model: input.model, host: "127.0.0.1", port: 11435, upstream_port: 11436 },
      input.jobId,
    ));
  });
  ipcMain.handle(IPC_CHANNELS.serverStop, async (event, raw: unknown) => {
    assertTrustedFrame(event);
    const input = ipcSchemas.serverStopInput.parse(raw);
    return ipcSchemas.serverMutationResult.parse(await controlClient.request(
      "server.stop", { id: input.id }, input.jobId,
    ));
  });
  ipcMain.handle(IPC_CHANNELS.serverStatus, async (event, raw: unknown) => {
    assertTrustedFrame(event);
    const input = ipcSchemas.serverStatusInput.parse(raw ?? {});
    return ipcSchemas.serverStatusResult.parse(await controlClient.request(
      "server.status", input.id ? { id: input.id } : {},
    ));
  });
  ipcMain.handle(IPC_CHANNELS.chatRequest, async (event, raw: unknown) => {
    assertTrustedFrame(event);
    const input = ipcSchemas.chatRequestInput.parse(raw);
    return ipcSchemas.chatResponseResult.parse(
      await controlClient.request(
        "chat.request",
        { model: input.model, port: 11435, messages: input.messages },
        input.jobId,
        null,
      ),
    );
  });
  ipcMain.handle(IPC_CHANNELS.selectWorkspace, async (event) => {
    assertTrustedFrame(event);
    const parent = BrowserWindow.fromWebContents(event.sender);
    const result = parent
      ? await dialog.showOpenDialog(parent, {
          properties: ["openDirectory"],
          title: "Choose an agent workspace",
        })
      : await dialog.showOpenDialog({
          properties: ["openDirectory"],
          title: "Choose an agent workspace",
        });
    if (result.canceled || !result.filePaths[0]) return undefined;
    const selectedPath = path.resolve(result.filePaths[0]);
    return ipcSchemas.workspaceResult.parse({
      path: selectedPath,
      displayName: path.basename(selectedPath),
    });
  });
  ipcMain.handle(IPC_CHANNELS.agentDescribe, async (event, raw: unknown) => {
    assertTrustedFrame(event);
    const input = ipcSchemas.agentDescribeInput.parse(raw);
    return ipcSchemas.agentDescribeResult.parse(
      await controlClient.request("agent.describe", input),
    );
  });
  ipcMain.handle(IPC_CHANNELS.agentLaunch, async (event, raw: unknown) => {
    assertTrustedFrame(event);
    const input = ipcSchemas.agentLaunchInput.parse(raw);
    const workspacePath = path.resolve(input.workspace.path);
    const workspaceInfo = await lstat(workspacePath);
    if (!workspaceInfo.isDirectory() || workspaceInfo.isSymbolicLink()) {
      throw new Error("Agent workspace must be a regular directory and not a symlink");
    }
    const descriptor = ipcSchemas.agentDescribeResult.parse(
      await controlClient.request("agent.describe", {
        agent: input.agent,
        model: input.model,
      }),
    );
    if (!descriptor.installed) {
      throw new Error(`${input.agent} is not installed`);
    }
    const cliPath = resolveCliExecutable({
      isPackaged: app.isPackaged,
      resourcesPath: process.resourcesPath,
      appPath: app.isPackaged ? app.getAppPath() : path.resolve(currentDirectory, ".."),
      platform: process.platform,
    });
    const cliInfo = await lstat(cliPath);
    if (!cliInfo.isFile() || cliInfo.isSymbolicLink()) {
      throw new Error("The Tapioca CLI launcher is unavailable");
    }
    const runId = crypto.randomUUID();
    const launchRoot = path.join(app.getPath("temp"), "tapioca-agent-launches");
    await import("node:fs/promises").then(({ mkdir }) => mkdir(launchRoot, { recursive: true }));
    if (process.platform === "win32") {
      const escapeBatch = (value: string) => value.replaceAll("%", "%%");
      const scriptPath = path.join(launchRoot, `${runId}.cmd`);
      await writeFile(scriptPath, [
        "@echo off",
        `cd /d "${escapeBatch(workspacePath)}"`,
        `"${escapeBatch(cliPath)}" launch "${input.agent}" "${input.model}"`,
        "pause",
        "",
      ].join("\r\n"), { mode: 0o700 });
      const error = await shell.openPath(scriptPath);
      if (error) throw new Error(error);
    } else {
      const quote = (value: string) => `'${value.replaceAll("'", `'\"'\"'`)}'`;
      const scriptPath = path.join(launchRoot, `${runId}.command`);
      await writeFile(scriptPath, [
        "#!/bin/sh",
        `cd ${quote(workspacePath)}`,
        `exec ${quote(cliPath)} launch ${quote(input.agent)} ${quote(input.model)}`,
        "",
      ].join("\n"), { mode: 0o700 });
      if (process.platform === "darwin") {
        const error = await shell.openPath(scriptPath);
        if (error) throw new Error(error);
      } else {
        const terminals: Array<[string, string[]]> = [
          ["/usr/bin/gnome-terminal", ["--", scriptPath]],
          ["/usr/bin/konsole", ["-e", scriptPath]],
          ["/usr/bin/x-terminal-emulator", ["-e", scriptPath]],
          ["/usr/bin/xterm", ["-e", scriptPath]],
        ];
        const terminal = terminals.find(([candidate]) => {
          try {
            return statSync(candidate).isFile();
          } catch {
            return false;
          }
        });
        if (!terminal) {
          throw new Error("No supported terminal application was found");
        }
        const child = spawn(terminal[0], terminal[1], {
          detached: true,
          stdio: "ignore",
          shell: false,
        });
        child.unref();
      }
    }
    return ipcSchemas.agentLaunchResult.parse({
      runId,
      message: `${descriptor.agent} opened in a terminal window.`,
    });
  });
  ipcMain.handle(IPC_CHANNELS.creatorCapabilities, async (event) => {
    assertTrustedFrame(event);
    return ipcSchemas.creatorCapabilitiesResult.parse(
      await controlClient.request("creator.capabilities"),
    );
  });
  ipcMain.handle(IPC_CHANNELS.creatorCatalog, async (event) => {
    assertTrustedFrame(event);
    return ipcSchemas.creatorCatalogResult.parse(
      await controlClient.request("creator.catalog"),
    );
  });
  ipcMain.handle(IPC_CHANNELS.creatorPickFile, async (event, raw: unknown) => {
    assertTrustedFrame(event);
    const { kind } = ipcSchemas.creatorPickFileInput.parse(raw);
    const filters = {
      image: [{ name: "Images", extensions: ["png", "jpg", "jpeg", "webp"] }],
      audio: [{ name: "Audio", extensions: ["wav", "mp3", "flac", "m4a", "ogg"] }],
      lora: [{ name: "LoRA adapters", extensions: ["safetensors"] }],
    }[kind];
    const parent = BrowserWindow.fromWebContents(event.sender);
    const options: OpenDialogOptions = {
      properties: ["openFile"],
      title: `Choose a ${kind} file`,
      filters,
    };
    const result = parent
      ? await dialog.showOpenDialog(parent, options)
      : await dialog.showOpenDialog(options);
    if (result.canceled || !result.filePaths[0]) return undefined;
    const selected = path.resolve(result.filePaths[0]);
    const info = await lstat(selected);
    if (!info.isFile() || info.isSymbolicLink()) {
      throw new Error("Selected input must be a regular file and not a symbolic link");
    }
    const allowed = {
      image: new Set([".png", ".jpg", ".jpeg", ".webp"]),
      audio: new Set([".wav", ".mp3", ".flac", ".m4a", ".ogg"]),
      lora: new Set([".safetensors"]),
    }[kind];
    if (!allowed.has(path.extname(selected).toLowerCase())) {
      throw new Error(`Unsupported ${kind} file type`);
    }
    const token = mediaRegistry.add(selected, kind);
    return ipcSchemas.creatorPickFileResult.parse({
      token,
      name: path.basename(selected),
      kind,
      ...(kind !== "lora" ? { previewUrl: mediaRegistry.url(token) } : {}),
    });
  });
  ipcMain.handle(IPC_CHANNELS.creatorSaveRecording, async (event, raw: unknown) => {
    assertTrustedFrame(event);
    const input = ipcSchemas.creatorSaveRecordingInput.parse(raw);
    const storage = zStorageInfo.parse(await controlClient.request("storage.info"));
    const directory = path.resolve(storage.home, "recordings");
    await mkdir(directory, { recursive: true, mode: 0o700 });
    const filePath = path.join(
      directory,
      `voice-reference-${Date.now()}-${crypto.randomUUID()}.wav`,
    );
    await writeFile(filePath, input.bytes, { flag: "wx", mode: 0o600 });
    const token = mediaRegistry.add(filePath, "audio", {
      durationSeconds: input.durationSeconds,
      source: "microphone",
    });
    return ipcSchemas.creatorSaveRecordingResult.parse({
      token,
      name: path.basename(filePath),
      kind: "audio",
      previewUrl: mediaRegistry.url(token),
    });
  });
  ipcMain.handle(IPC_CHANNELS.creatorGenerate, async (event, raw: unknown) => {
    assertTrustedFrame(event);
    const input = ipcSchemas.creatorGenerateInput.parse(raw);
    const activeServers = ipcSchemas.serverStatusResult.parse(
      await controlClient.request("server.status", {}),
    );
    for (const server of activeServers) {
      if (server.state === "stopped" || server.state === "failed") continue;
      await controlClient.request("server.stop", { id: server.id });
    }
    for (let attempt = 0; attempt < 120; attempt += 1) {
      const remaining = ipcSchemas.serverStatusResult.parse(
        await controlClient.request("server.status", {}),
      ).some((server) => server.state !== "stopped" && server.state !== "failed");
      if (!remaining) break;
      await new Promise((resolve) => setTimeout(resolve, 250));
      if (attempt === 119) {
        throw new Error("Timed out while releasing the chat model before generation");
      }
    }
    const isSpeech = input.mode === "speech" || input.mode === "voice-clone";
    const loras: Array<{ reference: string; scale: number }> = [];
    for (const lora of input.loras) {
      if (lora.type === "local") {
        const selected = mediaRegistry.get(lora.token, "lora");
        const imported = z.object({
          reference: z.string().min(4),
        }).passthrough().parse(await controlClient.request("lora.import", {
          path: selected.path,
          base: input.model,
        }));
        loras.push({ reference: imported.reference, scale: lora.weight });
        continue;
      }
      const reference = lora.reference.replace(/@-?\d+(?:\.\d+)?$/, "");
      const installed = z.object({
        reference: z.string().min(4),
      }).passthrough().parse(await controlClient.request("lora.pull", { reference }));
      loras.push({ reference: installed.reference, scale: lora.weight });
    }
    const common = {
      model: input.model,
      prompt: input.prompt,
      width: input.settings.width,
      height: input.settings.height,
      steps: input.settings.steps,
      ...(input.settings.seed === undefined ? {} : { seed: input.settings.seed }),
      loras,
    };
    const params = isSpeech
      ? {
          model: input.model,
          text: input.text?.trim() || input.prompt.trim(),
          voice_sample: input.voiceReferenceToken
            ? mediaRegistry.get(input.voiceReferenceToken, "audio").path
            : undefined,
        }
      : input.mode === "image"
        ? {
            ...common,
            input_images: input.inputImageToken
              ? [mediaRegistry.get(input.inputImageToken, "image").path]
              : [],
          }
        : {
            ...common,
            frames: input.settings.frames,
            fps: input.settings.fps,
            input_image: input.inputImageToken
              ? mediaRegistry.get(input.inputImageToken, "image").path
              : undefined,
          };
    const output = zCreatorOutput.parse(
      await controlClient.request(
        input.mode === "image"
          ? "image.generate"
          : input.mode === "video"
            ? "video.generate"
            : input.mode === "voice-clone"
              ? "voice.clone"
              : "speech.generate",
        params,
        input.jobId,
        null,
      ),
    );
    const storage = zStorageInfo.parse(await controlClient.request("storage.info"));
    const outputPath = path.resolve(output.path);
    const outputRoot = path.resolve(storage.home, "outputs");
    const relativeOutput = path.relative(outputRoot, outputPath);
    const outputInfo = await lstat(outputPath);
    if (
      relativeOutput.startsWith("..") ||
      path.isAbsolute(relativeOutput) ||
      !outputInfo.isFile() ||
      outputInfo.isSymbolicLink()
    ) {
      throw new Error("Runtime returned an unsafe output path");
    }
    const mediaType = input.mode === "image" ? "image" : input.mode === "video" ? "video" : "audio";
    const id = mediaRegistry.add(outputPath, mediaType, {
      model: output.model,
      prompt: isSpeech ? input.text ?? input.prompt : input.prompt,
      ...(isSpeech ? {} : {
        width: input.settings.width,
        height: input.settings.height,
      }),
      ...(input.mode === "video"
        ? { frames: input.settings.frames, fps: input.settings.fps }
        : {}),
      createdAt: output.created_at,
    });
    return ipcSchemas.creatorGenerateResult.parse({
      id,
      mode: input.mode,
      mediaType,
      url: mediaRegistry.url(id),
      createdAt: output.created_at,
      modelName: output.model,
      prompt: isSpeech ? input.text ?? input.prompt : input.prompt,
      metadata: mediaRegistry.get(id).metadata,
    });
  });
  ipcMain.handle(IPC_CHANNELS.creatorOutputs, async (event) => {
    assertTrustedFrame(event);
    const storage = zStorageInfo.parse(await controlClient.request("storage.info"));
    const outputRoot = path.resolve(storage.home, "outputs");
    const definitions = [
      { directory: "images", extension: ".png", mode: "image" as const, mediaType: "image" as const },
      { directory: "videos", extension: ".mp4", mode: "video" as const, mediaType: "video" as const },
      { directory: "audio", extension: ".wav", mode: "speech" as const, mediaType: "audio" as const },
    ];
    const outputs = [];
    for (const definition of definitions) {
      const directory = path.join(outputRoot, definition.directory);
      let names: string[];
      try {
        names = await readdir(directory);
      } catch {
        continue;
      }
      for (const name of names) {
        if (path.extname(name).toLowerCase() !== definition.extension) continue;
        const filePath = path.resolve(directory, name);
        const info = await lstat(filePath).catch(() => undefined);
        if (!info?.isFile() || info.isSymbolicLink()) continue;
        const id = mediaRegistry.add(filePath, definition.mediaType, {
          recovered: true,
          bytes: info.size,
        });
        outputs.push({
          id,
          mode: definition.mode,
          mediaType: definition.mediaType,
          url: mediaRegistry.url(id),
          createdAt: info.birthtime.toISOString(),
          modelName: "Local output",
          metadata: mediaRegistry.get(id).metadata ?? {},
        });
      }
    }
    outputs.sort((left, right) => right.createdAt.localeCompare(left.createdAt));
    return ipcSchemas.creatorOutputsResult.parse(outputs.slice(0, 100));
  });
  ipcMain.handle(IPC_CHANNELS.creatorLoraList, async (event) => {
    assertTrustedFrame(event);
    const rows = z.array(z.object({
      reference: z.string().min(4),
      provider: z.enum(["huggingface", "civitai", "modelscope", "local"]),
      file: z.string().min(1),
      path: z.string().min(1),
      bytes: z.number().int().nonnegative(),
      bases: z.array(z.string()).optional(),
    }).passthrough()).parse(await controlClient.request("lora.list"));
    return ipcSchemas.creatorLoraListResult.parse(
      rows.map((row) => ({
        id: crypto.randomUUID(),
        reference: row.reference,
        provider: row.provider,
        file: row.file,
        bytes: row.bytes,
        ...(row.bases ? { bases: row.bases } : {}),
      })),
    );
  });
  ipcMain.handle(IPC_CHANNELS.creatorLoraInspect, async (event, raw: unknown) => {
    assertTrustedFrame(event);
    const input = ipcSchemas.creatorLoraInspectInput.parse(raw);
    return ipcSchemas.creatorLoraInspectResult.parse(
      await controlClient.request("lora.inspect", { reference: input.reference.replace(/@-?\d+(?:\.\d+)?$/, "") }),
    );
  });
  ipcMain.handle(IPC_CHANNELS.creatorReveal, async (event, raw: unknown) => {
    assertTrustedFrame(event);
    const { outputId } = ipcSchemas.creatorOutputInput.parse(raw);
    shell.showItemInFolder(mediaRegistry.get(outputId).path);
  });
  ipcMain.handle(IPC_CHANNELS.creatorSaveMetadata, async (event, raw: unknown) => {
    assertTrustedFrame(event);
    const { outputId } = ipcSchemas.creatorOutputInput.parse(raw);
    const output = mediaRegistry.get(outputId);
    const parent = BrowserWindow.fromWebContents(event.sender);
    const options = {
      title: "Save generation metadata",
      defaultPath: `${path.basename(output.path, path.extname(output.path))}.json`,
      filters: [{ name: "JSON", extensions: ["json"] }],
    };
    const result = parent
      ? await dialog.showSaveDialog(parent, options)
      : await dialog.showSaveDialog(options);
    if (result.canceled || !result.filePath) return false;
    await writeFile(result.filePath, `${JSON.stringify(output.metadata ?? {}, null, 2)}\n`, {
      encoding: "utf8",
      flag: "wx",
    });
    return true;
  });
}

function registerMediaProtocol(): void {
  session.defaultSession.protocol.handle("tapioca-media", (request) => {
    const url = new URL(request.url);
    if (url.hostname !== "asset") return new Response("Not found", { status: 404 });
    try {
      const token = url.pathname.slice(1);
      const entry = mediaRegistry.get(token);
      return net.fetch(pathToFileURL(entry.path).href);
    } catch {
      return new Response("Not found", { status: 404 });
    }
  });
}

function createWindow(): BrowserWindow {
  const window = new BrowserWindow({
    width: 1440,
    height: 900,
    minWidth: 1080,
    minHeight: 680,
    backgroundColor: "#171317",
    title: "Tapioca",
    show: false,
    webPreferences: {
      preload: path.join(currentDirectory, "preload.cjs"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
      webSecurity: true,
      allowRunningInsecureContent: false,
    },
  });

  window.setMenuBarVisibility(false);
  const webContentsId = window.webContents.id;
  trustedWebContents.add(webContentsId);
  window.webContents.once("destroyed", () => {
    trustedWebContents.delete(webContentsId);
  });
  window.webContents.setWindowOpenHandler(() => ({ action: "deny" }));
  window.webContents.on("will-navigate", (event, navigationUrl) => {
    const currentUrl = window.webContents.getURL();
    if (navigationUrl !== currentUrl) {
      event.preventDefault();
    }
  });
  window.once("ready-to-show", () => window.show());

  if (process.env.VITE_DEV_SERVER_URL) {
    void window.loadURL(process.env.VITE_DEV_SERVER_URL);
  } else {
    void window.loadFile(rendererIndexPath);
  }

  return window;
}

app.whenReady().then(async () => {
  registerContentSecurityPolicy();
  registerMediaProtocol();
  registerIpcHandlers();
  controlClient.onEvent((event) => {
    for (const window of BrowserWindow.getAllWindows()) {
      const webContents = window.webContents;
      if (!webContents.isDestroyed() && trustedWebContents.has(webContents.id)) {
        webContents.send(IPC_CHANNELS.jobEvent, event);
      }
    }
  });
  const executable = resolveSidecarExecutable({
    isPackaged: app.isPackaged,
    resourcesPath: process.resourcesPath,
    appPath: app.isPackaged
      ? app.getAppPath()
      : path.resolve(currentDirectory, ".."),
    platform: process.platform,
  });
  sidecarExecutable = executable;
  const startup = await settleControlBeforeWindow(
    controlClient,
    executable,
    createWindow,
  );
  if (!startup.ready) {
    console.error("Tapioca control sidecar failed to start:", startup.error);
  }

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") {
    app.quit();
  }
});

app.on("before-quit", (event) => {
  if (shutdownStarted) return;
  event.preventDefault();
  shutdownStarted = true;
  void controlClient.stop().finally(() => app.exit(0));
});
