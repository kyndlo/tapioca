import path from "node:path";

export type MediaKind = "image" | "audio" | "lora" | "video";

interface MediaEntry {
  path: string;
  kind: MediaKind;
  metadata?: Record<string, unknown>;
}

export class MediaRegistry {
  readonly #entries = new Map<string, MediaEntry>();
  readonly #tokensByPath = new Map<string, string>();

  add(filePath: string, kind: MediaKind, metadata?: Record<string, unknown>) {
    const resolvedPath = path.resolve(filePath);
    const existingToken = this.#tokensByPath.get(resolvedPath);
    if (existingToken) {
      const existing = this.#entries.get(existingToken);
      if (existing?.kind === kind) {
        if (metadata) existing.metadata = { ...existing.metadata, ...metadata };
        return existingToken;
      }
    }
    const token = crypto.randomUUID();
    this.#entries.set(token, { path: resolvedPath, kind, metadata });
    this.#tokensByPath.set(resolvedPath, token);
    return token;
  }

  get(token: string, expected?: MediaKind): MediaEntry {
    const entry = this.#entries.get(token);
    if (!entry || (expected && entry.kind !== expected)) {
      throw new Error("Unknown or incompatible opaque media token");
    }
    return entry;
  }

  url(token: string): string {
    this.get(token);
    return `tapioca-media://asset/${token}`;
  }
}
