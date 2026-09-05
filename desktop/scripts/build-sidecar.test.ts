import path from "node:path";
import { describe, expect, it, vi } from "vitest";
import { buildSidecar, createSidecarBuildPlan } from "./build-sidecar";

describe("sidecar build workflow", () => {
  it("targets the exact development discovery path on Unix and Windows", () => {
    expect(
      createSidecarBuildPlan("/repo with spaces/desktop", "darwin"),
    ).toEqual({
      command: "go",
      args: [
        "build",
        "-o",
        path.resolve("/repo with spaces/bin/tapioca-control"),
        "./cmd/tapioca-control",
      ],
      cwd: path.resolve("/repo with spaces"),
      output: path.resolve("/repo with spaces/bin/tapioca-control"),
      env: expect.objectContaining({ GOOS: "darwin", GOARCH: process.arch === "x64" ? "amd64" : process.arch, CGO_ENABLED: "0" }),
    });
    expect(
      createSidecarBuildPlan("/repo/desktop", "win32", "x64"),
    ).toMatchObject({
      output: path.resolve("/repo/bin/tapioca-control.exe"),
      env: expect.objectContaining({ GOOS: "windows", GOARCH: "amd64" }),
    });
  });

  it("passes fixed arguments with shell interpolation disabled", () => {
    const plan = createSidecarBuildPlan("/repo/desktop", "linux");
    const run = vi.fn().mockReturnValue({ status: 0 });
    buildSidecar(plan, run, vi.fn());
    expect(run).toHaveBeenCalledWith(
      "go",
      ["build", "-o", path.resolve("/repo/bin/tapioca-control"), "./cmd/tapioca-control"],
      expect.objectContaining({
        cwd: path.resolve("/repo"),
        shell: false,
        env: expect.objectContaining({ GOOS: "linux" }),
      }),
    );
  });

  it("fails the npm workflow when Go compilation fails", () => {
    const plan = createSidecarBuildPlan("/repo/desktop", "linux");
    expect(() =>
      buildSidecar(plan, vi.fn().mockReturnValue({ status: 2 }), vi.fn()),
    ).toThrow("exit code 2");
  });
});
