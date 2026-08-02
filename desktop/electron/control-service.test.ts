import { describe, expect, it, vi } from "vitest";
import { ControlService } from "./control-service";

describe("ControlService", () => {
  it("calls catalog.list without params and applies filtering locally", async () => {
    const request = vi.fn().mockResolvedValue([
      {
        name: "gemma",
        kind: "text",
        backend: "llama.cpp",
        repo: "example/gemma",
      },
      {
        name: "qwen-image",
        kind: "image",
        backend: "diffusers",
        repo: "example/qwen-image",
      },
    ]);
    const service = new ControlService({ request });

    const result = await service.catalog({ kind: "image" });

    expect(request).toHaveBeenCalledTimes(1);
    expect(request).toHaveBeenCalledWith("catalog.list");
    expect(result.models.map((model) => model.name)).toEqual(["qwen-image"]);
  });
});
