import { mkdirSync } from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath, pathToFileURL } from "node:url";

export interface SidecarBuildPlan {
  command: string;
  args: readonly string[];
  cwd: string;
  output: string;
  env: NodeJS.ProcessEnv;
}

export function createSidecarBuildPlan(
  desktopRoot: string,
  platform: NodeJS.Platform,
  arch: NodeJS.Architecture = process.arch,
): SidecarBuildPlan {
  const repositoryRoot = path.resolve(desktopRoot, "..");
  const executable =
    platform === "win32" ? "tapioca-control.exe" : "tapioca-control";
  const output = path.resolve(repositoryRoot, "bin", executable);
  return {
    command: "go",
    args: ["build", "-o", output, "./cmd/tapioca-control"],
    cwd: repositoryRoot,
    output,
    env: {
      ...process.env,
      GOOS: platform === "win32" ? "windows" : platform === "darwin" ? "darwin" : "linux",
      GOARCH: arch === "x64" ? "amd64" : arch,
      CGO_ENABLED: "0",
    },
  };
}

export function buildSidecar(
  plan: SidecarBuildPlan,
  run = spawnSync,
  ensureDirectory: (directory: string) => void = (directory) =>
    mkdirSync(directory, { recursive: true }),
): void {
  ensureDirectory(path.dirname(plan.output));
  const result = run(plan.command, [...plan.args], {
    cwd: plan.cwd,
    shell: false,
    stdio: "inherit",
    windowsHide: true,
    env: plan.env,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`Go sidecar build failed with exit code ${result.status}`);
  }
}

const invokedPath = process.argv[1]
  ? pathToFileURL(path.resolve(process.argv[1])).href
  : "";
if (import.meta.url === invokedPath) {
  const desktopRoot = path.resolve(
    path.dirname(fileURLToPath(import.meta.url)),
    "..",
  );
  const targetPlatform = (process.env.TAPIOCA_TARGET_PLATFORM ?? process.env.npm_config_platform ?? process.platform) as NodeJS.Platform;
  const targetArch = (process.env.TAPIOCA_TARGET_ARCH ?? process.env.npm_config_arch ?? process.arch) as NodeJS.Architecture;
  if (!["darwin", "win32", "linux"].includes(targetPlatform)) {
    throw new Error(`Unsupported sidecar target platform: ${targetPlatform}`);
  }
  if (!["arm64", "x64"].includes(targetArch)) {
    throw new Error(`Unsupported sidecar target architecture: ${targetArch}`);
  }
  const plan = createSidecarBuildPlan(desktopRoot, targetPlatform, targetArch);
  buildSidecar(plan);
  console.log(`Built ${plan.output}`);
  const cliOutput = path.resolve(
    path.dirname(plan.output),
    targetPlatform === "win32" ? "tapioca.exe" : "tapioca",
  );
  buildSidecar({
    ...plan,
    args: ["build", "-o", cliOutput, "./cmd/tapioca"],
    output: cliOutput,
  });
  console.log(`Built ${cliOutput}`);
}
