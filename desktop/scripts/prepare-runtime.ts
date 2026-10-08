import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { verifyBundledRuntime } from "./verify-runtime.ts";

export const LLAMA_CPP_VERSION = "b11476";

const runtimeChecksums: Record<string, string> = {
  "darwin/arm64": "577634a1b8a59e8dabe02ba10de1e610be0574dfaf1cf3020e6dd42853ed877e",
  "win32/x64": "5c71e7b749697da4a8d46e9ee55486845cbba27c9dfbecb4007f31ba6610d523",
  "win32/arm64": "68e3a218ed7d9cd563e8ddf7a1e58d88d034a8f90a91061bdd3876bf247d8a93",
  "linux/x64": "5bb4306d7917f33e81efda02e6f791ae6a82e86bee121227a3ab2b4e8e40427f",
  "linux/arm64": "8dae2f39afee01d3032a101d7c398a6734d6fd9697690d8f93bdce4e3a9efea2",
};

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
  const destination = path.resolve(desktopRoot, "runtime", "llama.cpp");
  try {
    const preparedVersion = await readFile(path.join(destination, "tapioca-runtime-version.txt"), "utf8");
    if (preparedVersion.trim() !== LLAMA_CPP_VERSION) throw new Error("llama.cpp runtime is outdated");
    return verifyBundledRuntime(desktopRoot, platform);
  } catch {
    // Download below.
  }
  const asset = llamaRuntimeAsset(platform, arch);
  const temporary = path.join(os.tmpdir(), `tapioca-${asset.filename}`);
  const response = await fetch(asset.url, { redirect: "follow" });
  if (!response.ok) {
    throw new Error(`Download ${asset.url} failed: ${response.status} ${response.statusText}`);
  }
  const archive = Buffer.from(await response.arrayBuffer());
  const expected = runtimeChecksums[`${platform}/${arch}`];
  if (!expected || createHash("sha256").update(archive).digest("hex") !== expected) {
    throw new Error(`llama.cpp archive checksum mismatch for ${platform}/${arch}`);
  }
  await rm(destination, { recursive: true, force: true });
  await mkdir(destination, { recursive: true });
  await writeFile(temporary, archive);

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
  const runtimePath = verifyBundledRuntime(desktopRoot, platform);
  await writeFile(path.join(destination, "tapioca-runtime-version.txt"), `${LLAMA_CPP_VERSION}\n`);
  return runtimePath;
}

const invoked = process.argv[1] ? path.resolve(process.argv[1]) : "";
if (invoked === fileURLToPath(import.meta.url)) {
  const desktopRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
  console.log(`Prepared ${await prepareRuntime(desktopRoot)}`);
}
