import {
  catalogResultSchema,
  healthResultSchema,
  installedResultSchema,
  type CatalogFilter,
} from "../src/shared/ipc";
import {
  catalogListResultSchema,
  healthControlResultSchema,
  installedListResultSchema,
} from "../src/shared/sidecar";

function desktopKind(kind: string): "chat" | "image" | "video" | "speech" {
  const normalized = kind.toLowerCase();
  if (normalized === "image") return "image";
  if (normalized === "video") return "video";
  if (["speech", "audio", "tts", "voice"].includes(normalized)) return "speech";
  return "chat";
}

export function adaptHealthResult(value: unknown) {
  const health = healthControlResultSchema.parse(value);
  return healthResultSchema.parse({
    status: "ready",
    protocolVersion: health.protocol_version,
    platform: health.goos,
    arch: health.goarch,
    checkedAt: health.time,
    ...(health.control_version ? { controlVersion: health.control_version } : {}),
    ...(health.module_version ? { moduleVersion: health.module_version } : {}),
    ...(health.uptime_ms === undefined ? {} : { uptimeMs: health.uptime_ms }),
  });
}

export function adaptCatalogResult(value: unknown, filter: CatalogFilter) {
  const models = catalogListResultSchema
    .parse(value)
    .map((model) => ({
      id: model.name,
      name: model.name,
      kind: desktopKind(model.kind),
      description: [model.backend, model.memory].filter(Boolean).join(" · "),
      installed: false,
      tags: [model.backend, model.repo, ...(model.platforms ?? [])],
    }))
    .filter((model) => !filter.kind || model.kind === filter.kind);
  return catalogResultSchema.parse({ models });
}

export function adaptInstalledResult(value: unknown) {
  const models = installedListResultSchema.parse(value).map((model) => ({
    id: model.name,
    name: model.name,
    kind: desktopKind(model.kind),
    description: model.backend,
    installed: true as const,
    tags: [model.backend, model.repo],
  }));
  return installedResultSchema.parse({ models });
}
