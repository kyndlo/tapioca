import type {
  CompatibilityResult,
  MachineProfile,
  ModelRecord,
} from "./types";

export function modelCompatibility(
  model: ModelRecord,
  machine: MachineProfile,
): CompatibilityResult {
  const reasons: string[] = [];
  if (!model.requirements.platforms.includes(machine.platform)) {
    reasons.push(`Not available for ${machine.platform}`);
  }
  if (machine.memoryBytes < model.requirements.memoryBytes) {
    reasons.push("Not enough memory");
  }
  if (!model.installed && machine.availableDiskBytes < model.requirements.diskBytes) {
    reasons.push("Not enough disk space");
  }
  if (
    !model.requirements.accelerators.some((accelerator) =>
      machine.accelerators.includes(accelerator),
    )
  ) {
    reasons.push("No supported accelerator");
  }
  if (reasons.length) return { level: "incompatible", reasons };

  const recommended =
    model.requirements.recommendedMemoryBytes ??
    model.requirements.memoryBytes * 1.25;
  if (machine.memoryBytes < recommended) {
    return {
      level: "tight",
      reasons: ["Runs on this machine, with limited headroom"],
    };
  }
  return { level: "compatible", reasons: ["Good fit for this machine"] };
}

export function downloadPercent(received: number, total: number): number | undefined {
  if (!Number.isFinite(total) || total <= 0 || !Number.isFinite(received)) return undefined;
  return Math.max(0, Math.min(100, Math.round(received / total * 100)));
}

export function formatModelBytes(bytes: number): string {
  const gib = bytes / 1024 ** 3;
  return `${gib >= 10 ? Math.round(gib) : gib.toFixed(1)} GB`;
}

export function estimatedDiskAfterInstall(
  model: ModelRecord,
  machine: MachineProfile,
): { required: string; remaining: string } {
  return {
    required: formatModelBytes(model.requirements.diskBytes),
    remaining: formatModelBytes(
      Math.max(0, machine.availableDiskBytes - (model.installed ? 0 : model.requirements.diskBytes)),
    ),
  };
}
