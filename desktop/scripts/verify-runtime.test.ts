import path from "node:path";
import { describe, expect, it, vi } from "vitest";
import { verifyBundledRuntime } from "./verify-runtime";

describe("packaged model runtime", () => {
  it("requires the platform llama-server before packaging", () => {
    expect(() => verifyBundledRuntime("/missing", "darwin")).toThrow(
      "Missing",
    );
    const desktopRoot = path.resolve(import.meta.dirname, "..");
    expect(verifyBundledRuntime(desktopRoot, "darwin")).toBe(
      path.join(desktopRoot, "runtime", "llama.cpp", "llama-server"),
    );
  });
});
