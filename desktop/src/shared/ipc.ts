import { z } from "zod";
import {
  catalogModelSchema,
  controlEventSchema,
} from "./sidecar";

export const modelKindSchema = z.enum(["chat", "image", "video", "speech"]);
export const desktopModelSchema = z
  .object({
    id: z.string().min(1),
    name: z.string().min(1),
    kind: modelKindSchema,
    description: z.string(),
    installed: z.boolean(),
    tags: z.array(z.string()),
  })
  .strict();
export const catalogFilterSchema = z
  .object({ kind: modelKindSchema.optional() })
  .strict();
export const catalogResultSchema = z
  .object({ models: z.array(desktopModelSchema) })
  .strict();
export const installedResultSchema = z
  .object({
    models: z.array(desktopModelSchema.extend({ installed: z.literal(true) })),
  })
  .strict();
export type CatalogFilter = z.infer<typeof catalogFilterSchema>;
export const healthResultSchema = z
  .object({
    status: z.enum(["ready", "starting", "degraded"]),
    protocolVersion: z.literal(1),
    platform: z.string().min(1),
    arch: z.string().min(1),
    checkedAt: z.string().datetime(),
    controlVersion: z.string().min(1).optional(),
    moduleVersion: z.string().min(1).optional(),
    uptimeMs: z.number().int().nonnegative().optional(),
  })
  .strict();

export const systemSnapshotSchema = z
  .object({
    platform: z.enum(["macos", "windows", "linux"]),
    arch: z.string().min(1),
    cpuCount: z.number().int().positive(),
    accelerators: z.array(z.enum(["apple", "nvidia", "amd", "intel", "cpu"])),
    memoryBytes: z.number().int().nonnegative(),
    modelsPath: z.string().min(1),
    modelsBytes: z.number().int().nonnegative(),
    availableDiskBytes: z.number().int().nonnegative(),
  })
  .strict();

export const catalogRefreshResultSchema = z.object({
  path: z.string().min(1),
  models: z.number().int().positive(),
  sha256: z.string().regex(/^[a-f0-9]{64}$/),
}).strict();

export const softwareUpdateInfoSchema = z.object({
  currentVersion: z.string().min(1),
  latestVersion: z.string().min(1),
  available: z.boolean(),
  releaseUrl: z.string().url(),
}).strict();
export const softwareUpdateInstallResultSchema = z.object({
  started: z.boolean(),
  message: z.string().min(1),
}).strict();

export const modelCatalogResultSchema = z
  .object({
    catalog: z.array(catalogModelSchema),
    installed: z.array(
      z
        .object({
          name: z.string().min(1),
          repo: z.string().min(1),
          filename: z.string().optional(),
          kind: z.string().min(1),
          backend: z.string().min(1),
        })
        .strict(),
    ),
  })
  .strict();

const jobInputBase = {
  jobId: z.string().min(1).max(128),
};
export const modelPullInputSchema = z
	.object({
		...jobInputBase,
		name: z.string().min(1).max(256),
		acceptLicense: z.boolean().optional(),
		accessToken: z.string().min(1).max(1024).optional(),
	})
  .strict();
export const cancelJobInputSchema = z
  .object({ jobId: z.string().min(1).max(128) })
  .strict();
export const modelRemoveInputSchema = z
  .object({
    name: z.string().min(1).max(256),
    confirm: z.string().min(1).max(256),
  })
  .strict();

export const serverStatusSchema = z
  .object({
    id: z.string(),
    model: z.string(),
    endpoint: z.string().url(),
    state: z.enum(["starting", "running", "stopping", "stopped", "failed"]),
    error: z.string().optional(),
  })
  .strict();
export const serverStatusesSchema = z
  .array(serverStatusSchema)
  .nullable()
  .transform((value) => value ?? []);
export const serverStartInputSchema = z.object({
  jobId: z.string().min(1).max(128),
  model: z.string().min(1).max(256),
  id: z.string().min(1).max(128),
}).strict();
export const serverStopInputSchema = z.object({
  jobId: z.string().min(1).max(128),
  id: z.string().min(1).max(128),
}).strict();
export const serverStatusInputSchema = z.object({
  id: z.string().min(1).max(128).optional(),
}).strict();

export const chatStatusResultSchema = z
  .object({
    installed: modelCatalogResultSchema.shape.installed,
    servers: serverStatusesSchema,
  })
  .strict();
export const chatRequestInputSchema = z
  .object({
    jobId: z.string().min(1).max(128),
    model: z.string().min(1).max(256),
    messages: z
      .array(
        z
          .object({
            role: z.enum(["user", "assistant"]),
            content: z.string(),
          })
          .strict(),
      )
      .min(1)
      .max(512),
  })
  .strict();
export const chatResponseSchema = z
  .object({
    id: z.string(),
    object: z.string(),
    created: z.number(),
    model: z.string(),
    choices: z.array(
      z
        .object({
          index: z.number().int(),
          message: z
            .object({
              role: z.string(),
              content: z.unknown().optional(),
              reasoning_content: z.string().optional(),
              tool_calls: z.array(z.object({
                id: z.string().min(1).max(256),
                type: z.literal("function"),
                function: z.object({
                  name: z.string().min(1).max(256),
                  arguments: z.string().max(64 * 1024),
                }).strict(),
              }).strict()).max(64).optional(),
            })
            .passthrough(),
          finish_reason: z.string(),
        })
        .passthrough(),
    ),
    usage: z.record(z.unknown()).optional(),
  })
  .passthrough();

export const workspaceSelectionSchema = z
  .object({ path: z.string().min(1), displayName: z.string().min(1) })
  .strict();
export const agentDescribeInputSchema = z
  .object({
    agent: z.enum(["codex", "claude", "opencode", "openclaw", "hermes"]),
    model: z.string().min(1).max(256),
  })
  .strict();
export const agentDescriptorSchema = z
  .object({
    agent: z.string().min(1),
    executable: z.string().min(1),
    args: z.array(z.string()),
    environment: z.record(z.string()),
    configuration: z.record(z.unknown()).optional(),
    endpoint: z.string().url(),
    protocol: z.string().min(1),
    installed: z.boolean(),
  })
  .strict();
export const agentLaunchInputSchema = z.object({
  agent: agentDescribeInputSchema.shape.agent,
  model: z.string().min(1).max(256),
  workspace: workspaceSelectionSchema,
}).strict();
export const agentLaunchResultSchema = z.object({
  runId: z.string().uuid(),
  message: z.string().min(1),
}).strict();

const creatorCapabilitySchema = z
  .object({
    available: z.boolean(),
    method: z.string().min(1),
    supports_input_images: z.boolean().optional(),
    supports_input_image: z.boolean().optional(),
    supports_loras: z.boolean().optional(),
    error_code: z.string().optional(),
  })
  .passthrough();
export const creatorCapabilitiesSchema = z
  .object({
    image: creatorCapabilitySchema,
    video: creatorCapabilitySchema,
    speech: creatorCapabilitySchema,
    voice_clone: creatorCapabilitySchema,
    outputs: z
      .object({
        binary_in_protocol: z.boolean(),
        managed_local_paths: z.boolean(),
      })
      .strict(),
    progress: z
      .object({
        mode: z.string().min(1),
        numeric_when_available: z.boolean(),
        reason: z.string().optional(),
      })
      .passthrough()
      .optional(),
  })
  .strict();
export const creatorCatalogItemSchema = catalogModelSchema.extend({
  operation: z.string().min(1),
  supports_input_image: z.boolean(),
  requires_input_image: z.boolean(),
  supports_lora: z.boolean(),
  available: z.boolean(),
  unavailable_reason: z.string().optional(),
});
export const creatorCatalogSchema = z.array(creatorCatalogItemSchema);
export const creatorFileKindSchema = z.enum(["image", "audio", "lora"]);
export const creatorPickFileInputSchema = z.object({ kind: creatorFileKindSchema }).strict();
export const creatorFileSelectionSchema = z
  .object({
    token: z.string().uuid(),
    name: z.string().min(1),
    kind: creatorFileKindSchema,
    previewUrl: z.string().url().optional(),
  })
  .strict();
export const creatorSaveRecordingInputSchema = z
  .object({
    bytes: z.instanceof(Uint8Array).refine((value) => value.byteLength > 0 && value.byteLength <= 50 * 1024 * 1024, {
      message: "Voice recording must be between 1 byte and 50 MiB",
    }),
    durationSeconds: z.number().positive().max(300),
  })
  .strict();
const creatorLoraSchema = z.discriminatedUnion("type", [
  z.object({
    type: z.literal("reference"),
    reference: z.string().min(4).max(2048).regex(/^(?:hf|ms|modelscope|civitai|local):\/\//),
    weight: z.number().min(-4).max(4),
  }).strict(),
  z.object({
    type: z.literal("local"),
    token: z.string().uuid(),
    weight: z.number().min(-4).max(4),
  }).strict(),
]);
export const creatorGenerateInputSchema = z
  .object({
    jobId: z.string().min(1).max(128),
    mode: z.enum(["image", "video", "speech", "voice-clone"]),
    model: z.string().min(1).max(256),
    prompt: z.string().max(20_000),
    text: z.string().max(20_000).optional(),
    inputImageToken: z.string().uuid().optional(),
    voiceReferenceToken: z.string().uuid().optional(),
    voiceConsent: z.boolean().optional(),
    transcript: z.string().max(20_000).optional(),
    loras: z.array(creatorLoraSchema).max(8),
    settings: z.object({
      width: z.number().int().min(64).max(4096),
      height: z.number().int().min(64).max(4096),
      steps: z.number().int().min(1).max(200),
      seed: z.number().int().nonnegative().optional(),
      frames: z.number().int().min(1).max(513),
      fps: z.number().int().min(1).max(60),
    }).strict(),
  })
  .strict();
export const creatorOutputSchema = z.object({
  id: z.string().uuid(),
  mode: z.enum(["image", "video", "speech", "voice-clone"]),
  mediaType: z.enum(["image", "video", "audio"]),
  url: z.string().url(),
  createdAt: z.string().datetime(),
  modelName: z.string().min(1),
  prompt: z.string().optional(),
  metadata: z.record(z.union([z.string(), z.number(), z.boolean()])),
}).strict();
export const creatorOutputInputSchema = z.object({ outputId: z.string().uuid() }).strict();
export const creatorLoraInspectInputSchema = z.object({
  reference: z.string().min(4).max(2048).regex(/^(?:hf|ms|modelscope|civitai|local):\/\//),
}).strict();
export const creatorLoraListSchema = z.array(z.object({
  id: z.string().min(1),
  reference: z.string().min(4),
  provider: z.enum(["huggingface", "civitai", "modelscope", "local"]),
  file: z.string().min(1),
  bytes: z.number().int().nonnegative(),
  bases: z.array(z.string()).optional(),
}).strict());

export const IPC_CHANNELS = {
  health: "tapioca:health",
  systemSnapshot: "tapioca:system-snapshot",
  models: "tapioca:models",
  catalogRefresh: "tapioca:catalog-refresh",
  softwareUpdateCheck: "tapioca:software-update-check",
  softwareUpdateInstall: "tapioca:software-update-install",
  modelPull: "tapioca:model-pull",
  modelRemove: "tapioca:model-remove",
  cancelJob: "tapioca:cancel-job",
  chatStatus: "tapioca:chat-status",
  serverStart: "tapioca:server-start",
  serverStop: "tapioca:server-stop",
  serverStatus: "tapioca:server-status",
  chatRequest: "tapioca:chat-request",
  selectWorkspace: "tapioca:select-workspace",
  agentDescribe: "tapioca:agent-describe",
  agentLaunch: "tapioca:agent-launch",
  creatorCapabilities: "tapioca:creator-capabilities",
  creatorCatalog: "tapioca:creator-catalog",
  creatorPickFile: "tapioca:creator-pick-file",
  creatorSaveRecording: "tapioca:creator-save-recording",
  creatorGenerate: "tapioca:creator-generate",
  creatorOutputs: "tapioca:creator-outputs",
  creatorLoraList: "tapioca:creator-lora-list",
  creatorLoraInspect: "tapioca:creator-lora-inspect",
  creatorReveal: "tapioca:creator-reveal",
  creatorSaveMetadata: "tapioca:creator-save-metadata",
  jobEvent: "tapioca:job-event",
} as const;

export const ipcSchemas = {
  healthResult: healthResultSchema,
  systemSnapshotResult: systemSnapshotSchema,
  modelsResult: modelCatalogResultSchema,
  catalogRefreshResult: catalogRefreshResultSchema,
  softwareUpdateInfoResult: softwareUpdateInfoSchema,
  softwareUpdateInstallResult: softwareUpdateInstallResultSchema,
  modelPullInput: modelPullInputSchema,
  modelPullResult: modelCatalogResultSchema.shape.installed.element,
  modelRemoveInput: modelRemoveInputSchema,
  cancelJobInput: cancelJobInputSchema,
  chatStatusResult: chatStatusResultSchema,
  serverStartInput: serverStartInputSchema,
  serverStopInput: serverStopInputSchema,
  serverStatusInput: serverStatusInputSchema,
  serverStatusResult: serverStatusesSchema,
  serverMutationResult: serverStatusSchema,
  chatRequestInput: chatRequestInputSchema,
  chatResponseResult: chatResponseSchema,
  workspaceResult: workspaceSelectionSchema.optional(),
  agentDescribeInput: agentDescribeInputSchema,
  agentDescribeResult: agentDescriptorSchema,
  agentLaunchInput: agentLaunchInputSchema,
  agentLaunchResult: agentLaunchResultSchema,
  creatorCapabilitiesResult: creatorCapabilitiesSchema,
  creatorCatalogResult: creatorCatalogSchema,
  creatorPickFileInput: creatorPickFileInputSchema,
  creatorPickFileResult: creatorFileSelectionSchema.optional(),
  creatorSaveRecordingInput: creatorSaveRecordingInputSchema,
  creatorSaveRecordingResult: creatorFileSelectionSchema,
  creatorGenerateInput: creatorGenerateInputSchema,
  creatorGenerateResult: creatorOutputSchema,
  creatorOutputsResult: z.array(creatorOutputSchema),
  creatorLoraListResult: creatorLoraListSchema,
  creatorLoraInspectInput: creatorLoraInspectInputSchema,
  creatorLoraInspectResult: z.record(z.unknown()),
  creatorOutputInput: creatorOutputInputSchema,
  jobEvent: controlEventSchema,
} as const;

export interface TapiocaDesktopApi {
  health(): Promise<z.infer<typeof healthResultSchema>>;
  systemSnapshot(): Promise<z.infer<typeof systemSnapshotSchema>>;
  models(): Promise<z.infer<typeof modelCatalogResultSchema>>;
  catalogRefresh(): Promise<z.infer<typeof catalogRefreshResultSchema>>;
  softwareUpdateCheck(): Promise<z.infer<typeof softwareUpdateInfoSchema>>;
  softwareUpdateInstall(): Promise<z.infer<typeof softwareUpdateInstallResultSchema>>;
  modelPull(input: z.infer<typeof modelPullInputSchema>): Promise<z.infer<typeof modelCatalogResultSchema>["installed"][number]>;
  modelRemove(input: z.infer<typeof modelRemoveInputSchema>): Promise<void>;
  cancelJob(input: z.infer<typeof cancelJobInputSchema>): Promise<boolean>;
  chatStatus(): Promise<z.infer<typeof chatStatusResultSchema>>;
  serverStart(input: z.infer<typeof serverStartInputSchema>): Promise<z.infer<typeof serverStatusSchema>>;
  serverStop(input: z.infer<typeof serverStopInputSchema>): Promise<z.infer<typeof serverStatusSchema>>;
  serverStatus(input?: z.infer<typeof serverStatusInputSchema>): Promise<Array<z.infer<typeof serverStatusSchema>>>;
  chatRequest(input: z.infer<typeof chatRequestInputSchema>): Promise<z.infer<typeof chatResponseSchema>>;
  selectWorkspace(): Promise<z.infer<typeof workspaceSelectionSchema> | undefined>;
  agentDescribe(input: z.infer<typeof agentDescribeInputSchema>): Promise<z.infer<typeof agentDescriptorSchema>>;
  agentLaunch(input: z.infer<typeof agentLaunchInputSchema>): Promise<z.infer<typeof agentLaunchResultSchema>>;
  creatorCapabilities(): Promise<z.infer<typeof creatorCapabilitiesSchema>>;
  creatorCatalog(): Promise<z.infer<typeof creatorCatalogSchema>>;
  creatorPickFile(input: z.infer<typeof creatorPickFileInputSchema>): Promise<z.infer<typeof creatorFileSelectionSchema> | undefined>;
  creatorSaveRecording(input: z.infer<typeof creatorSaveRecordingInputSchema>): Promise<z.infer<typeof creatorFileSelectionSchema>>;
  creatorGenerate(input: z.infer<typeof creatorGenerateInputSchema>): Promise<z.infer<typeof creatorOutputSchema>>;
  creatorOutputs(): Promise<Array<z.infer<typeof creatorOutputSchema>>>;
  creatorLoraList(): Promise<z.infer<typeof creatorLoraListSchema>>;
  creatorLoraInspect(input: z.infer<typeof creatorLoraInspectInputSchema>): Promise<Record<string, unknown>>;
  creatorReveal(input: z.infer<typeof creatorOutputInputSchema>): Promise<void>;
  creatorSaveMetadata(input: z.infer<typeof creatorOutputInputSchema>): Promise<boolean>;
  onJobEvent(listener: (event: z.infer<typeof controlEventSchema>) => void): () => void;
}
