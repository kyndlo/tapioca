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
