import { createHash } from "node:crypto";
import { createReadStream, createWriteStream } from "node:fs";
import { chmod, mkdir, readFile, readdir, rename, rm, writeFile } from "node:fs/promises";
import { execFile, spawn } from "node:child_process";
import { get } from "node:https";
import path from "node:path";
import { pipeline } from "node:stream/promises";
import { Transform } from "node:stream";

const RELEASE_API = "https://api.github.com/repos/kyndlo/tapioca/releases/latest";

type ReleaseAsset = { name: string; browser_download_url: string };
type Release = {
  tag_name: string;
  html_url: string;
  draft: boolean;
  assets: ReleaseAsset[];
};

export type DesktopUpdateInfo = {
  currentVersion: string;
  latestVersion: string;
  available: boolean;
  releaseUrl: string;
  asset?: ReleaseAsset;
  checksumAsset?: ReleaseAsset;
};

function versionParts(value: string): [number, number, number] | undefined {
  const match = value.replace(/^v/, "").match(/^(\d+)\.(\d+)\.(\d+)$/);
  return match ? [Number(match[1]), Number(match[2]), Number(match[3])] : undefined;
}

export function isNewerVersion(candidate: string, current: string): boolean {
  const left = versionParts(candidate);
  const right = versionParts(current);
  if (!left || !right) return false;
  for (let index = 0; index < 3; index += 1) {
    if (left[index] !== right[index]) return left[index] > right[index];
  }
  return false;
}

export function desktopAssetName(platform: NodeJS.Platform, arch: string): string | undefined {
  if (platform === "darwin" && arch === "arm64") return "tapioca-desktop-macos-arm64.zip";
  if (platform === "win32" && arch === "x64") return "tapioca-desktop-windows-amd64.exe";
  if (platform === "win32" && arch === "arm64") return "tapioca-desktop-windows-arm64.exe";
  if (platform === "linux" && arch === "x64") return "tapioca-desktop-linux-amd64.AppImage";
  if (platform === "linux" && arch === "arm64") return "tapioca-desktop-linux-arm64.AppImage";
  return undefined;
}

export async function checkDesktopUpdate(currentVersion: string): Promise<DesktopUpdateInfo> {
  const response = await fetch(process.env.TAPIOCA_RELEASE_API ?? RELEASE_API, {
    headers: { Accept: "application/vnd.github+json", "User-Agent": `tapioca-desktop/${currentVersion}` },
  });
  if (!response.ok) throw new Error(`GitHub release check failed: ${response.status} ${response.statusText}`);
  const release = await response.json() as Release;
  const latestVersion = release.tag_name?.replace(/^v/, "");
  if (release.draft || !versionParts(latestVersion)) throw new Error("Latest GitHub release has an invalid version");
  const available = isNewerVersion(latestVersion, currentVersion);
  const info: DesktopUpdateInfo = {
    currentVersion,
    latestVersion,
    available,
    releaseUrl: release.html_url,
  };
  if (!available) return info;
  const name = desktopAssetName(process.platform, process.arch);
  if (!name) throw new Error(`Desktop updates are unsupported on ${process.platform}/${process.arch}`);
  info.asset = release.assets.find((asset) => asset.name === name);
  info.checksumAsset = release.assets.find((asset) => asset.name === `${name}.sha256`);
  if (!info.asset || !info.checksumAsset) {
    throw new Error(`Release v${latestVersion} does not contain ${name} and its checksum`);
  }
  return info;
}

async function download(url: string, destination: string, redirects = 5): Promise<void> {
  const parsed = new URL(url);
  if (parsed.protocol !== "https:") throw new Error("Update downloads must use HTTPS");
  await new Promise<void>((resolve, reject) => {
    const request = get(parsed, { headers: { "User-Agent": "tapioca-desktop-updater" } }, (response) => {
      if (response.statusCode && response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
        response.resume();
        if (redirects <= 0) return reject(new Error("Too many update download redirects"));
        void download(new URL(response.headers.location, parsed).href, destination, redirects - 1).then(resolve, reject);
        return;
      }
      if (response.statusCode !== 200) {
        response.resume();
        reject(new Error(`Update download failed: HTTP ${response.statusCode ?? "unknown"}`));
        return;
      }
      const maximumBytes = 2 * 1024 * 1024 * 1024;
      let received = 0;
      const limiter = new Transform({
        transform(chunk, _encoding, callback) {
          received += chunk.length;
          callback(received > maximumBytes ? new Error("Update download exceeds the 2 GiB safety limit") : undefined, chunk);
        },
      });
      void pipeline(response, limiter, createWriteStream(destination, { mode: 0o600 })).then(resolve, reject);
    });
    request.once("error", reject);
  });
}

async function sha256(file: string): Promise<string> {
  const digest = createHash("sha256");
  for await (const chunk of createReadStream(file)) digest.update(chunk);
  return digest.digest("hex");
}

export async function installDesktopUpdate(
  info: DesktopUpdateInfo,
  temporaryRoot: string,
): Promise<{ started: boolean; message: string; shouldQuit: boolean; shouldRelaunch: boolean }> {
  if (!info.available || !info.asset || !info.checksumAsset) {
    return { started: false, message: "Tapioca is already current.", shouldQuit: false, shouldRelaunch: false };
  }
  const root = path.join(temporaryRoot, `tapioca-desktop-${info.latestVersion}`);
  await rm(root, { recursive: true, force: true });
  await mkdir(root, { recursive: true });
  const checksumPath = path.join(root, `${info.asset.name}.sha256`);
  const assetPath = path.join(root, info.asset.name);
  await download(info.checksumAsset.browser_download_url, checksumPath);
  const expected = (await readFile(checksumPath, "utf8")).trim().split(/\s+/)[0];
  if (!/^[a-fA-F0-9]{64}$/.test(expected)) throw new Error("Release checksum is invalid");
  await download(info.asset.browser_download_url, assetPath);
  if ((await sha256(assetPath)).toLowerCase() !== expected.toLowerCase()) {
    await rm(assetPath, { force: true });
    throw new Error("Downloaded desktop update checksum did not match");
  }

  if (process.platform === "linux" && process.env.APPIMAGE) {
    const current = process.env.APPIMAGE;
    const replacement = `${current}.update`;
    await rename(assetPath, replacement);
    await chmod(replacement, 0o755);
    await rename(replacement, current);
    const helper = path.join(root, "restart.sh");
    await writeFile(helper, `#!/bin/sh\nwhile kill -0 ${process.pid} 2>/dev/null; do sleep 1; done\nrm -rf ${shellQuote(root)}\nexec ${shellQuote(current)}\n`, { mode: 0o700 });
    const child = spawn("/bin/sh", [helper], { detached: true, stdio: "ignore" });
    child.unref();
    return { started: true, message: "Update installed. Restarting Tapioca…", shouldQuit: true, shouldRelaunch: false };
  }

  if (process.platform === "win32") {
    const child = spawn(assetPath, ["/S"], { detached: true, stdio: "ignore" });
    child.unref();
    return { started: true, message: "Verified installer started. Tapioca will close to finish the update.", shouldQuit: true, shouldRelaunch: false };
  }

  if (process.platform === "darwin") {
    const extracted = path.join(root, "extracted");
    await mkdir(extracted, { recursive: true });
    await execute("/usr/bin/ditto", ["-x", "-k", assetPath, extracted]);
    const appName = (await readdir(extracted)).find((name) => name.endsWith(".app"));
    if (!appName) throw new Error("macOS update archive does not contain a Tapioca app");
    const currentApp = path.resolve(path.dirname(process.execPath), "../..");
    if (!currentApp.endsWith(".app")) throw new Error("Cannot safely identify the current macOS app bundle");
    const stagedApp = path.join(extracted, appName);
    const helper = path.join(root, "install.sh");
    const backup = `${currentApp}.previous`;
    const script = `#!/bin/sh
while kill -0 ${process.pid} 2>/dev/null; do sleep 1; done
rm -rf ${shellQuote(backup)}
mv ${shellQuote(currentApp)} ${shellQuote(backup)} || exit 1
if mv ${shellQuote(stagedApp)} ${shellQuote(currentApp)}; then
  rm -rf ${shellQuote(backup)}
  rm -rf ${shellQuote(root)}
  open ${shellQuote(currentApp)}
else
  mv ${shellQuote(backup)} ${shellQuote(currentApp)}
  exit 1
fi
`;
    await writeFile(helper, script, { mode: 0o700 });
    const child = spawn("/bin/sh", [helper], { detached: true, stdio: "ignore" });
    child.unref();
    return { started: true, message: "Verified update is staged. Tapioca will restart when installation finishes.", shouldQuit: true, shouldRelaunch: false };
  }

  throw new Error(`Desktop updates are unsupported on ${process.platform}/${process.arch}`);
}

function shellQuote(value: string): string {
  return `'${value.replaceAll("'", `'"'"'`)}'`;
}

function execute(command: string, args: string[]): Promise<void> {
  return new Promise((resolve, reject) => {
    execFile(command, args, (error) => error ? reject(error) : resolve());
  });
}
