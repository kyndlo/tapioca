import { describe, expect, it } from "vitest";
import { desktopAssetName, isNewerVersion } from "./app-updater";

describe("desktop updater", () => {
  it("compares semantic release versions", () => {
    expect(isNewerVersion("0.9.0", "0.8.9")).toBe(true);
    expect(isNewerVersion("0.8.0", "0.8.0")).toBe(false);
    expect(isNewerVersion("bad", "0.8.0")).toBe(false);
  });

  it("maps supported platforms to stable release assets", () => {
    expect(desktopAssetName("darwin", "arm64")).toBe("tapioca-desktop-macos-arm64.zip");
    expect(desktopAssetName("win32", "x64")).toBe("tapioca-desktop-windows-amd64.exe");
    expect(desktopAssetName("linux", "arm64")).toBe("tapioca-desktop-linux-arm64.AppImage");
  });
});
