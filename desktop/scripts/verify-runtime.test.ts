import path from "node:path";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { describe, expect, it, vi } from "vitest";
import { verifyBundledRuntime } from "./verify-runtime";

describe("packaged model runtime", () => {
  it("requires the platform llama-server before packaging", () => {
    expect(() => verifyBundledRuntime("/missing", "darwin")).toThrow(
      "Missing",
    );
    const desktopRoot = mkdtempSync(path.join(tmpdir(), "tapioca-runtime-test-"));
    try {
      const runtime = path.join(desktopRoot, "runtime", "llama.cpp");
      mkdirSync(runtime, { recursive: true });
      for (const [platform, name] of [["darwin", "llama-server"], ["win32", "llama-server.exe"]] as const) {
        writeFileSync(path.join(runtime, name), "test fixture");
        expect(verifyBundledRuntime(desktopRoot, platform)).toBe(path.join(runtime, name));
      }
    } finally { rmSync(desktopRoot, { recursive: true, force: true }); }
  });
});
