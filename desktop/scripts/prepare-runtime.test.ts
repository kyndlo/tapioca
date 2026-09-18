import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { describe, expect, it, vi } from "vitest";
import { LLAMA_CPP_VERSION, llamaRuntimeAsset, prepareRuntime, windowsExtraction } from "./prepare-runtime";

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

  it("preserves an existing runtime if the replacement archive fails verification", async () => {
    const root = await mkdtemp(path.join(os.tmpdir(), "tapioca-runtime-test-"));
    const runtime = path.join(root, "runtime", "llama.cpp");
    const executable = path.join(runtime, "llama-server");
    await mkdir(runtime, { recursive: true });
    await writeFile(executable, "existing runtime");
    await writeFile(path.join(runtime, "tapioca-runtime-version.txt"), "b10603\n");
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      arrayBuffer: async () => new TextEncoder().encode("invalid archive").buffer,
    });
    vi.stubGlobal("fetch", fetchMock);
    try {
      await expect(prepareRuntime(root, "darwin", "arm64")).rejects.toThrow("checksum mismatch");
      expect(fetchMock).toHaveBeenCalledOnce();
      expect(await readFile(executable, "utf8")).toBe("existing runtime");
    } finally {
      vi.unstubAllGlobals();
      await rm(root, { recursive: true, force: true });
    }
  });
});
