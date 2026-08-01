import assert from "node:assert/strict";
import { access, readFile } from "node:fs/promises";
import test from "node:test";

async function render() {
  const workerUrl = new URL("../dist/server/index.js", import.meta.url);
  workerUrl.searchParams.set("test", `${process.pid}-${Date.now()}`);
  const { default: worker } = await import(workerUrl.href);
  return worker.fetch(
    new Request("https://tapioca.example/", { headers: { accept: "text/html" } }),
    { ASSETS: { fetch: async () => new Response("Not found", { status: 404 }) } },
    { waitUntil() {}, passThroughOnException() {} },
  );
}

test("renders the Tapioca documentation homepage", async () => {
  const response = await render();
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type") ?? "", /^text\/html\b/i);

  const html = await response.text();
  assert.match(html, /<title>Tapioca — Your local models, ready to roll<\/title>/i);
  assert.match(html, /Your models\./);
  assert.match(html, /From zero to chatting/);
  assert.match(html, /Windows ARM64/);
  assert.match(html, /OpenAI-compatible API/);
  assert.match(html, /tapioca run qwen3:8b-q4_k_m/);
  assert.match(html, /property="og:image"/);
  assert.doesNotMatch(html, /codex-preview|react-loading-skeleton|Your site is taking shape/i);
});

test("ships production brand assets and metadata", async () => {
  const [layout, page, packageJson] = await Promise.all([
    readFile(new URL("../app/layout.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/page.tsx", import.meta.url), "utf8"),
    readFile(new URL("../package.json", import.meta.url), "utf8"),
    access(new URL("../public/tapioca.png", import.meta.url)),
    access(new URL("../public/favicon.png", import.meta.url)),
    access(new URL("../public/og.png", import.meta.url)),
  ]);

  assert.match(layout, /summary_large_image/);
  assert.match(layout, /x-forwarded-host/);
  assert.match(page, /tapioca-local-ai|github\.com\/kyndlo\/tapioca/i);
  assert.doesNotMatch(packageJson, /react-loading-skeleton/);
});
