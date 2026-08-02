import { describe, expect, it } from "vitest";
import {
  adaptCatalogResult,
  adaptHealthResult,
  adaptInstalledResult,
} from "./control-adapters";

describe("control-to-renderer adapters", () => {
  it("normalizes canonical health without inferring hardware labels", () => {
    expect(
      adaptHealthResult({
        status: "ok",
        protocol_version: 1,
        goos: "darwin",
        goarch: "arm64",
        time: "2026-08-01T12:00:00Z",
        control_version: "0.2.0",
        module_version: "v0.4.0",
        uptime_ms: 42,
      }),
    ).toEqual({
      status: "ready",
      protocolVersion: 1,
      platform: "darwin",
      arch: "arm64",
      checkedAt: "2026-08-01T12:00:00Z",
      controlVersion: "0.2.0",
      moduleVersion: "v0.4.0",
      uptimeMs: 42,
    });
  });

  it("normalizes and filters canonical catalog results", () => {
    const result = adaptCatalogResult(
      [
        {
          name: "gemma3",
          kind: "text",
          backend: "mlx-vlm",
          repo: "example/gemma",
          memory: "16 GiB",
          platforms: ["macos"],
        },
        {
          name: "qwen-image",
          kind: "image",
          backend: "diffusers",
          repo: "example/image",
        },
      ],
      { kind: "image" },
    );
    expect(result.models).toHaveLength(1);
    expect(result.models[0]).toMatchObject({
      id: "qwen-image",
      kind: "image",
      installed: false,
    });
  });

  it("marks installed models without exposing their filesystem path", () => {
    const result = adaptInstalledResult([
      {
        name: "gemma3",
        repo: "example/gemma",
        path: "/private/models/gemma3",
        kind: "text",
        backend: "mlx-vlm",
      },
    ]);
    expect(result.models[0]).toMatchObject({
      kind: "chat",
      installed: true,
    });
    expect(JSON.stringify(result)).not.toContain("/private/models");
  });
});
