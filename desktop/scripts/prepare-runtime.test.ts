import { describe, expect, it } from "vitest";
import { LLAMA_CPP_VERSION, llamaRuntimeAsset } from "./prepare-runtime";

describe("llama.cpp runtime assets", () => {
  it("pins native runtime archives for every desktop target", () => {
    expect(llamaRuntimeAsset("darwin", "arm64").filename).toBe(
      `llama-${LLAMA_CPP_VERSION}-bin-macos-arm64.tar.gz`,
    );
    expect(llamaRuntimeAsset("win32", "x64").filename).toContain("win-vulkan-x64");
    expect(llamaRuntimeAsset("win32", "arm64").filename).toContain("win-cpu-arm64");
    expect(llamaRuntimeAsset("linux", "x64").filename).toContain("ubuntu-vulkan-x64");
    expect(llamaRuntimeAsset("linux", "arm64").filename).toContain("ubuntu-vulkan-arm64");
  });

  it("rejects unsupported desktop targets", () => {
    expect(() => llamaRuntimeAsset("darwin", "x64")).toThrow(
      "No bundled llama.cpp runtime",
    );
  });
});
