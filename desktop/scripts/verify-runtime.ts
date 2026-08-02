import { existsSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

export function verifyBundledRuntime(
  desktopRoot: string,
  platform: NodeJS.Platform = process.platform,
): string {
  const executable = platform === "win32" ? "llama-server.exe" : "llama-server";
  const runtime = path.resolve(desktopRoot, "runtime", "llama.cpp", executable);
  if (!existsSync(runtime) || !statSync(runtime).isFile()) {
    throw new Error(
      `Missing ${runtime}. Download the official llama.cpp runtime for this platform before packaging Tapioca.`,
    );
  }
  return runtime;
}

const invoked = process.argv[1] ? path.resolve(process.argv[1]) : "";
if (invoked === fileURLToPath(import.meta.url)) {
  const desktopRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
  console.log(`Verified ${verifyBundledRuntime(desktopRoot)}`);
}
