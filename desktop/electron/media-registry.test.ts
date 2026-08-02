import { describe, expect, it } from "vitest";
import { MediaRegistry } from "./media-registry";

describe("MediaRegistry", () => {
  it("keeps paths behind opaque, kind-checked tokens", () => {
    const registry = new MediaRegistry();
    const token = registry.add("/private/input.png", "image");
    expect(token).not.toContain("input.png");
    expect(registry.url(token)).toBe(`tapioca-media://asset/${token}`);
    expect(registry.get(token, "image").path).toMatch(/input\.png$/);
    expect(() => registry.get(token, "audio")).toThrow(/opaque media token/);
    expect(() => registry.get(crypto.randomUUID())).toThrow(/opaque media token/);
  });

  it("reuses opaque tokens when recovering the same managed output", () => {
    const registry = new MediaRegistry();
    const first = registry.add("/outputs/image.png", "image", { recovered: true });
    const second = registry.add("/outputs/image.png", "image", { model: "qwen" });
    expect(second).toBe(first);
    expect(registry.get(first).metadata).toEqual({ recovered: true, model: "qwen" });
  });
});
