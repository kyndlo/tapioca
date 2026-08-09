export interface RendererTrustPolicy {
  developmentServerUrl?: string;
  packagedRendererUrl: string;
}

export function isTrustedRendererUrl(
  candidate: string,
  policy: RendererTrustPolicy,
): boolean {
  let parsed: URL;
  try {
    parsed = new URL(candidate);
  } catch {
    return false;
  }

  if (policy.developmentServerUrl) {
    return parsed.origin === new URL(policy.developmentServerUrl).origin;
  }
  return (
    candidate === policy.packagedRendererUrl ||
    candidate.startsWith(`${policy.packagedRendererUrl}#`)
  );
}

export function isAudioCapturePermission(
  permission: string,
  mediaTypes: readonly string[] | undefined,
): boolean {
  if (permission !== "media" || !mediaTypes?.length) return false;
  return mediaTypes.every((type) => type === "audio");
}
