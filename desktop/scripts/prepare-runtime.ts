import { mkdir, rm, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { spawnSync } from "node:child_process";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { verifyBundledRuntime } from "./verify-runtime.ts";

export const LLAMA_CPP_VERSION = "b10603";

export function windowsExtraction(archive: string, destination: string) {
  return {
    args: ["-NoProfile", "-NonInteractive", "-Command",
      "Expand-Archive -LiteralPath $env:TAPIOCA_RUNTIME_ARCHIVE -DestinationPath $env:TAPIOCA_RUNTIME_DESTINATION -Force"],
    env: { ...process.env, TAPIOCA_RUNTIME_ARCHIVE: archive, TAPIOCA_RUNTIME_DESTINATION: destination },
  };
}

export function llamaRuntimeAsset(
  platform: NodeJS.Platform,
  arch: NodeJS.Architecture,
): { filename: string; url: string; zip: boolean } {
  let filename: string;
  if (platform === "darwin" && arch === "arm64") {
    filename = `llama-${LLAMA_CPP_VERSION}-bin-macos-arm64.tar.gz`;
  } else if (platform === "win32" && arch === "x64") {
    filename = `llama-${LLAMA_CPP_VERSION}-bin-win-vulkan-x64.zip`;
  } else if (platform === "win32" && arch === "arm64") {
    filename = `llama-${LLAMA_CPP_VERSION}-bin-win-cpu-arm64.zip`;
  } else if (platform === "linux" && arch === "x64") {
    filename = `llama-${LLAMA_CPP_VERSION}-bin-ubuntu-vulkan-x64.tar.gz`;
  } else if (platform === "linux" && arch === "arm64") {
    filename = `llama-${LLAMA_CPP_VERSION}-bin-ubuntu-vulkan-arm64.tar.gz`;
  } else {
    throw new Error(`No bundled llama.cpp runtime for ${platform}/${arch}`);
  }
  return {
    filename,
    url: `https://github.com/ggml-org/llama.cpp/releases/download/${LLAMA_CPP_VERSION}/${filename}`,
    zip: filename.endsWith(".zip"),
  };
}

export async function prepareRuntime(
  desktopRoot: string,
  platform: NodeJS.Platform = process.platform,
  arch: NodeJS.Architecture = process.arch,
): Promise<string> {
  try {
    return verifyBundledRuntime(desktopRoot, platform);
  } catch {
    // Download below.
  }
  const asset = llamaRuntimeAsset(platform, arch);
  const destination = path.resolve(desktopRoot, "runtime", "llama.cpp");
  const temporary = path.join(os.tmpdir(), `tapioca-${asset.filename}`);
  const response = await fetch(asset.url, { redirect: "follow" });
  if (!response.ok) {
    throw new Error(`Download ${asset.url} failed: ${response.status} ${response.statusText}`);
  }
  await rm(destination, { recursive: true, force: true });
  await mkdir(destination, { recursive: true });
  await writeFile(temporary, Buffer.from(await response.arrayBuffer()));

  const windows = windowsExtraction(temporary, destination);
  const extraction = asset.zip
    ? spawnSync(
        "powershell.exe",
        windows.args,
        { shell: false, stdio: "inherit", env: windows.env },
      )
    : spawnSync(
        "tar",
        ["-xzf", temporary, "-C", destination, "--strip-components=1"],
        { shell: false, stdio: "inherit" },
      );
  await rm(temporary, { force: true });
  if (extraction.error) throw extraction.error;
  if (extraction.status !== 0) {
    throw new Error(`Extracting llama.cpp failed with exit code ${extraction.status}`);
  }
  return verifyBundledRuntime(desktopRoot, platform);
}

const invoked = process.argv[1] ? path.resolve(process.argv[1]) : "";
if (invoked === fileURLToPath(import.meta.url)) {
  const desktopRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
  console.log(`Prepared ${await prepareRuntime(desktopRoot)}`);
}
