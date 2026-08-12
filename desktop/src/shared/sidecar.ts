import { z } from "zod";

export const SIDECAR_PROTOCOL_VERSION = 1 as const;
export const MAX_NDJSON_LINE_BYTES = 4 * 1024 * 1024;

export const controlMethodSchema = z.enum([
  "handshake",
  "capabilities.get",
  "health.get",
  "system.info",
  "storage.info",
  "catalog.list",
  "catalog.get",
  "installed.list",
  "model.pull",
  "model.remove",
  "server.start",
  "server.stop",
  "server.status",
  "chat.request",
  "chat.describe",
  "agent.describe",
  "creator.capabilities",
  "creator.catalog",
  "image.generate",
  "video.generate",
  "speech.generate",
  "voice.clone",
  "lora.list",
  "lora.inspect",
  "lora.pull",
  "lora.import",
  "job.cancel",
]);

const envelopeBase = {
  version: z.literal(SIDECAR_PROTOCOL_VERSION),
};

export const controlRequestSchema = z
  .object({
    ...envelopeBase,
    type: z.literal("request"),
    id: z.string().min(1).max(128),
    method: controlMethodSchema,
    params: z.unknown().optional(),
    job_id: z.string().min(1).max(128).optional(),
  })
  .strict();

const controlErrorSchema = z
  .object({
    code: z.string().min(1),
    message: z.string().min(1),
    retryable: z.boolean(),
    details: z.unknown().optional(),
  })
  .strict();

export const controlResponseSchema = z
  .object({
    ...envelopeBase,
    type: z.literal("response"),
    id: z.string().max(128),
    result: z.unknown().optional(),
    error: controlErrorSchema.optional(),
  })
  .strict()
  .superRefine((response, context) => {
    if ((response.result === undefined) === (response.error === undefined)) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        message: "response must contain exactly one of result or error",
      });
    }
    if (
      response.id.length === 0 &&
      (!response.error || response.error.code !== "invalid_request")
    ) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["id"],
        message: "only uncorrelated invalid_request errors may have an empty id",
      });
    }
  });

export const controlEventSchema = z
  .object({
    ...envelopeBase,
    type: z.literal("event"),
    event: z.enum([
      "job.started",
      "job.progress",
      "job.log",
      "job.completed",
      "job.failed",
    ]),
    job_id: z.string().min(1).max(128),
    sequence: z.number().int().min(1),
    timestamp: z.string().datetime(),
    data: z.unknown().optional(),
  })
  .strict();

export const controlEnvelopeSchema = z.union([
  controlResponseSchema,
  controlEventSchema,
]);

export const capabilitySetSchema = z
  .object({
    methods: z.array(controlMethodSchema),
    events: z.array(
      z.enum([
        "job.started",
        "job.progress",
        "job.log",
        "job.completed",
        "job.failed",
      ]),
    ),
    max_request_bytes: z.number().int().positive(),
    max_concurrency: z.number().int().positive(),
  })
  .strict();

export const handshakeResultSchema = z
  .object({
    name: z.literal("tapioca-control"),
    protocol_version: z.literal(SIDECAR_PROTOCOL_VERSION),
    capabilities: capabilitySetSchema,
  })
  .strict();

export const capabilitiesResultSchema = capabilitySetSchema;

export const healthControlResultSchema = z
  .object({
    status: z.literal("ok"),
    protocol_version: z.literal(SIDECAR_PROTOCOL_VERSION),
    goos: z.string().min(1),
    goarch: z.string().min(1),
    time: z.string().datetime(),
    name: z.string().min(1).optional(),
    control_version: z.string().min(1).optional(),
    module_version: z.string().min(1).optional(),
    go_version: z.string().min(1).optional(),
    started_at: z.string().datetime().optional(),
    uptime_ms: z.number().int().nonnegative().optional(),
  })
  .strict();

export const catalogModelSchema = z
  .object({
    name: z.string().min(1),
    kind: z.string().min(1),
    backend: z.string().min(1),
    repo: z.string().min(1),
    size: z.string().optional(),
    memory: z.string().optional(),
    gpu: z.string().optional(),
    platforms: z.array(z.string()).default([]),
    languages: z.string().optional(),
    features: z.string().optional(),
    width: z.number().int().positive().optional(),
    height: z.number().int().positive().optional(),
    steps: z.number().int().positive().optional(),
    frames: z.number().int().positive().optional(),
    fps: z.number().int().positive().optional(),
		gated: z.boolean().optional(),
		license: z.string().optional(),
		license_url: z.string().url().optional(),
  })
  .passthrough();

export const installedModelSchema = z
  .object({
    name: z.string().min(1),
    repo: z.string().min(1),
    path: z.string().min(1),
    kind: z.string().min(1),
    backend: z.string().min(1),
  })
  .passthrough();

export const catalogListResultSchema = z.array(catalogModelSchema);
export const installedListResultSchema = z.array(installedModelSchema);

export type ControlMethod = z.infer<typeof controlMethodSchema>;
export type ControlRequest = z.infer<typeof controlRequestSchema>;
export type ControlResponse = z.infer<typeof controlResponseSchema>;
export type ControlEvent = z.infer<typeof controlEventSchema>;

export function encodeRequest(request: ControlRequest): string {
  return `${JSON.stringify(controlRequestSchema.parse(request))}\n`;
}

export class NdjsonProtocolError extends Error {
  constructor(
    message: string,
    readonly code: "line_too_large" | "malformed_json" | "invalid_envelope",
  ) {
    super(message);
    this.name = "NdjsonProtocolError";
  }
}

export class BoundedNdjsonParser {
  private buffer = Buffer.alloc(0);

  constructor(
    private readonly maxLineBytes = MAX_NDJSON_LINE_BYTES,
  ) {}

  push(chunk: Buffer | string): Array<z.infer<typeof controlEnvelopeSchema>> {
    const incoming = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
    this.buffer = Buffer.concat([this.buffer, incoming]);
    const parsed: Array<z.infer<typeof controlEnvelopeSchema>> = [];

    while (true) {
      const newline = this.buffer.indexOf(0x0a);
      if (newline < 0) break;
      if (newline > this.maxLineBytes) {
        throw new NdjsonProtocolError(
          `NDJSON line exceeded ${this.maxLineBytes} bytes`,
          "line_too_large",
        );
      }
      const line = this.buffer.subarray(0, newline).toString("utf8").trim();
      this.buffer = this.buffer.subarray(newline + 1);
      if (line) parsed.push(this.parse(line));
    }

    if (this.buffer.byteLength > this.maxLineBytes) {
      throw new NdjsonProtocolError(
        `NDJSON line exceeded ${this.maxLineBytes} bytes`,
        "line_too_large",
      );
    }
    return parsed;
  }

  finish(): Array<z.infer<typeof controlEnvelopeSchema>> {
    const line = this.buffer.toString("utf8").trim();
    this.buffer = Buffer.alloc(0);
    return line ? [this.parse(line)] : [];
  }

  private parse(line: string): z.infer<typeof controlEnvelopeSchema> {
    let value: unknown;
    try {
      value = JSON.parse(line);
    } catch {
      throw new NdjsonProtocolError("Malformed JSON from sidecar", "malformed_json");
    }
    const parsed = controlEnvelopeSchema.safeParse(value);
    if (!parsed.success) {
      throw new NdjsonProtocolError(
        `Invalid sidecar envelope: ${parsed.error.message}`,
        "invalid_envelope",
      );
    }
    return parsed.data;
  }
}
