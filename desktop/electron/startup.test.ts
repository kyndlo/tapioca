import { describe, expect, it, vi } from "vitest";
import { settleControlBeforeWindow } from "./startup";

describe("sidecar startup gate", () => {
  it("creates the window only after a successful handshake settles", async () => {
    const order: string[] = [];
    let release!: () => void;
    const start = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          release = () => {
            order.push("ready");
            resolve();
          };
        }),
    );
    const createWindow = vi.fn(() => order.push("window"));
    const pending = settleControlBeforeWindow(
      { start, state: { status: "starting" } },
      "/bin/tapioca-control",
      createWindow,
    );
    expect(createWindow).not.toHaveBeenCalled();
    release();
    await expect(pending).resolves.toEqual({ ready: true });
    expect(order).toEqual(["ready", "window"]);
  });

  it("settles a failed startup before creating an observable offline window", async () => {
    const error = new Error("ENOENT");
    const createWindow = vi.fn();
    const result = await settleControlBeforeWindow(
      {
        start: vi.fn().mockRejectedValue(error),
        state: { status: "crashed", reason: "ENOENT" },
      },
      "/missing/tapioca-control",
      createWindow,
    );
    expect(result).toEqual({ ready: false, error });
    expect(createWindow).toHaveBeenCalledOnce();
  });
});
