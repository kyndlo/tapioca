import { describe, expect, it } from "vitest";
import { LLAMA_CPP_VERSION, llamaRuntimeAsset, windowsExtraction } from "./prepare-runtime";

describe("llama.cpp runtime assets", () => {
  it("passes Windows archive paths as data, including spaces and shell metacharacters", () => {
    const archive = "C:\\User's Files\\$runtime;test.zip";
    const destination = "C:\\Program Files\\Tapioca";
    const extraction = windowsExtraction(archive, destination);
    expect(extraction.env.TAPIOCA_RUNTIME_ARCHIVE).toBe(archive);
    expect(extraction.env.TAPIOCA_RUNTIME_DESTINATION).toBe(destination);
    expect(extraction.args.join(" ")).not.toContain(archive);
    expect(extraction.args.at(-1)).toContain("$env:TAPIOCA_RUNTIME_ARCHIVE");
  });
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
