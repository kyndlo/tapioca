import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { resolveSidecarExecutable } from "./control-client";

interface DesktopPackage {
  build: {
    files: string[];
    extraResources: Array<{
      from: string;
      to: string;
      filter: string[];
    }>;
  };
}

describe("Electron package layout", () => {
  const manifest = JSON.parse(
    readFileSync(path.resolve(process.cwd(), "package.json"), "utf8"),
  ) as DesktopPackage;

  it("packages renderer, main, preload, and the control sidecar", () => {
    expect(manifest.build.files).toEqual(
      expect.arrayContaining(["dist/renderer/**/*", "dist-electron/**/*"]),
    );
    expect(manifest.build.extraResources).toContainEqual({
      from: "../bin",
      to: "sidecar",
      filter: ["tapioca-control", "tapioca-control.exe", "tapioca", "tapioca.exe"],
    });
    expect(manifest.build.extraResources).toContainEqual({
      from: "runtime/llama.cpp",
      to: "sidecar/runtime/llama.cpp",
      filter: ["**/*"],
    });
  });

  it("discovers the exact packaged resource destination", () => {
    expect(
      resolveSidecarExecutable({
        isPackaged: true,
        resourcesPath: "/opt/Tapioca/resources",
        appPath: "/ignored",
        platform: "linux",
      }),
    ).toBe("/opt/Tapioca/resources/sidecar/tapioca-control");
  });
});
