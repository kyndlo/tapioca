import { describe, expect, it } from "vitest";
import { isTrustedRendererUrl } from "./security";

describe("renderer trust policy", () => {
  const packaged =
    "file:///Applications/Tapioca.app/Contents/Resources/app.asar/dist/renderer/index.html";

  it("allows only the exact packaged renderer and its hash routes", () => {
    expect(
      isTrustedRendererUrl(packaged, { packagedRendererUrl: packaged }),
    ).toBe(true);
    expect(
      isTrustedRendererUrl(`${packaged}#/models`, {
        packagedRendererUrl: packaged,
      }),
    ).toBe(true);
    expect(
      isTrustedRendererUrl("file:///tmp/attacker.html", {
        packagedRendererUrl: packaged,
      }),
    ).toBe(false);
    expect(
      isTrustedRendererUrl(`${packaged}.evil`, {
        packagedRendererUrl: packaged,
      }),
    ).toBe(false);
  });

  it("allows the configured development origin but no other origin", () => {
    expect(
      isTrustedRendererUrl("http://127.0.0.1:5173/#/chat", {
        developmentServerUrl: "http://127.0.0.1:5173",
        packagedRendererUrl: packaged,
      }),
    ).toBe(true);
    expect(
      isTrustedRendererUrl("http://localhost:5173/#/chat", {
        developmentServerUrl: "http://127.0.0.1:5173",
        packagedRendererUrl: packaged,
      }),
    ).toBe(false);
  });
});
